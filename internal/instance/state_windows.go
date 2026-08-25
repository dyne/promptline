//go:build windows

package instance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
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
type Lock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func (i *Instance) lockPath() string  { return filepath.Join(i.stateDir, "instance.lock") }
func (i *Instance) statePath() string { return filepath.Join(i.stateDir, "state.json") }

func (i *Instance) AcquireLock() (*Lock, error) {
	f, err := os.OpenFile(i.lockPath(), os.O_CREATE|os.O_RDWR, privateFileMode)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(privateFileMode); err != nil {
		_ = f.Close()
		return nil, err
	}
	lock := &Lock{file: f}
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &lock.overlapped)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("%w: %s", ErrInstanceLocked, i.name)
		}
		return nil, err
	}
	return lock, nil
}

func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	if err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &l.overlapped); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (i *Instance) LoadState() (State, error) {
	f, err := os.Open(i.statePath())
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
	tmp, err := os.CreateTemp(i.stateDir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(privateFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return i.stateRename(tmpPath, i.statePath())
}
