package governance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const EventSchemaVersion = 1

type Event struct {
	SchemaVersion int            `json:"schemaVersion"`
	Time          time.Time      `json:"time"`
	Instance      string         `json:"instance"`
	ActorUID      int            `json:"actorUid"`
	ActorGID      int            `json:"actorGid"`
	ProcessID     int            `json:"processId"`
	ThreadID      string         `json:"threadId,omitempty"`
	TurnID        string         `json:"turnId,omitempty"`
	RequestID     string         `json:"requestId,omitempty"`
	Kind          string         `json:"kind"`
	Decision      string         `json:"decision,omitempty"`
	Outcome       string         `json:"outcome,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type JournalConfig struct {
	Directory       string
	MaxRecordBytes  int
	MaxJournalBytes int64
	Retention       int
}

// Journal serializes append-only JSONL records. An append is fsynced at each
// caller-selected security boundary; it is never read to make authorization.
type Journal struct {
	mu     sync.Mutex
	cfg    JournalConfig
	path   string
	file   *os.File
	closed bool
}

func OpenJournal(cfg JournalConfig) (*Journal, error) {
	if cfg.Directory == "" || !filepath.IsAbs(cfg.Directory) {
		return nil, errors.New("journal directory must be absolute")
	}
	if cfg.MaxRecordBytes <= 0 {
		cfg.MaxRecordBytes = 64 << 10
	}
	if cfg.MaxJournalBytes <= 0 {
		cfg.MaxJournalBytes = 16 << 20
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 3
	}
	if err := os.MkdirAll(cfg.Directory, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(cfg.Directory, 0o700); err != nil {
		return nil, err
	}
	j := &Journal{cfg: cfg, path: filepath.Join(cfg.Directory, "events.jsonl")}
	if err := j.open(); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *Journal) open() error {
	f, err := os.OpenFile(j.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	j.file = f
	return nil
}

func (j *Journal) Append(event Event, syncBoundary bool) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return errors.New("journal is closed")
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = EventSchemaVersion
	}
	if event.SchemaVersion != EventSchemaVersion || event.Instance == "" || event.Kind == "" {
		return errors.New("invalid audit event")
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	} else {
		event.Time = event.Time.UTC()
	}
	event.Metadata = sanitizeMetadata(event.Metadata)
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(data)+1 > j.cfg.MaxRecordBytes {
		return errors.New("audit event exceeds record limit")
	}
	if err := j.rotateLocked(int64(len(data) + 1)); err != nil {
		return err
	}
	if _, err := j.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	if syncBoundary {
		return j.file.Sync()
	}
	return nil
}

func (j *Journal) rotateLocked(incoming int64) error {
	info, err := j.file.Stat()
	if err != nil {
		return err
	}
	if info.Size()+incoming <= j.cfg.MaxJournalBytes {
		return nil
	}
	if err := j.file.Sync(); err != nil {
		return err
	}
	if err := j.file.Close(); err != nil {
		return err
	}
	for n := j.cfg.Retention - 1; n >= 1; n-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", j.path, n), fmt.Sprintf("%s.%d", j.path, n+1))
	}
	if err := os.Rename(j.path, j.path+".1"); err != nil {
		return err
	}
	return j.open()
}

func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	return j.file.Close()
}

func sanitizeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "password") {
			out[key] = "[REDACTED]"
			continue
		}
		if text, ok := value.(string); ok {
			out[key] = strings.Map(func(r rune) rune {
				if r < 0x20 && r != '\t' {
					return -1
				}
				return r
			}, text)
		} else {
			out[key] = value
		}
	}
	return out
}

// RecoverJournal ignores one incomplete final line left by a crash. It rejects
// malformed complete records so corruption remains diagnosable.
func RecoverJournal(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		var event Event
		if err := json.Unmarshal(bytes.TrimSpace(s.Bytes()), &event); err != nil {
			return fmt.Errorf("invalid audit record: %w", err)
		}
	}
	return s.Err()
}
