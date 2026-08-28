//go:build unix

package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const stateSchemaVersion = 1

var ErrInstanceLocked = errors.New("promptline instance is already owned")

// State is the deliberately small Promptline-owned durable record.
type State struct {
	SchemaVersion       int    `json:"schemaVersion"`
	LastPrimaryThreadID string `json:"lastPrimaryThreadId,omitempty"`
	CodexVersion        string `json:"codexVersion,omitempty"`
	ProtocolVersion     string `json:"protocolVersion,omitempty"`
}

// Lock is an advisory, process-lifetime exclusive instance lock.
type Lock struct{ file *os.File }

func (i *Instance) lockPath() string  { return filepath.Join(i.stateDir, "instance.lock") }
func (i *Instance) statePath() string { return filepath.Join(i.stateDir, "state.json") }

func (i *Instance) AcquireLock() (*Lock, error) {
	if err := i.rejectUnsafeArtifact("instance.lock"); err != nil {
		return nil, err
	}
	f, err := i.stateRootHandle.OpenFile("instance.lock", os.O_CREATE|os.O_RDWR|syscall.O_CLOEXEC, privateFileMode)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(privateFileMode); err != nil {
		f.Close()
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%w: %s", ErrInstanceLocked, i.name)
		}
		return nil, err
	}
	return &Lock{file: f}, nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (i *Instance) LoadState() (State, error) {
	if err := i.rejectUnsafeArtifact("state.json"); err != nil {
		return State{}, err
	}
	f, err := i.stateRootHandle.Open("state.json")
	if errors.Is(err, os.ErrNotExist) {
		return State{SchemaVersion: stateSchemaVersion}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("invalid instance state: %w", err)
	}
	if state.SchemaVersion != stateSchemaVersion {
		return State{}, fmt.Errorf("unsupported instance state schema %d", state.SchemaVersion)
	}
	return state, nil
}

func (i *Instance) rejectUnsafeArtifact(name string) error {
	info, err := i.stateRootHandle.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe private state artifact %q", name)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private state artifact %q has unsafe mode", name)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() || stat.Nlink != 1 {
		return fmt.Errorf("unsafe private state artifact %q identity", name)
	}
	return nil
}

func (i *Instance) SaveState(state State) error {
	if state.SchemaVersion == 0 {
		state.SchemaVersion = stateSchemaVersion
	}
	if state.SchemaVersion != stateSchemaVersion {
		return fmt.Errorf("unsupported instance state schema %d", state.SchemaVersion)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	dir := i.stateDir
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(privateFileMode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := i.stateRename(tmpPath, i.statePath()); err != nil {
		return err
	}
	// Syncing the directory makes the rename durable on Unix filesystems.
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
