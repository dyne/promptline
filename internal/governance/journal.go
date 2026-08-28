package governance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const EventSchemaVersion = 1

const redacted = "[REDACTED]"

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
	Sequence      uint64         `json:"sequence"`
	PreviousHash  string         `json:"previousHash,omitempty"`
	Hash          string         `json:"hash"`
}

type JournalConfig struct {
	// Root is an already-open instance capability. When supplied, Directory is
	// a canonical relative directory below it; no host path is reconstructed.
	Root            *os.Root
	Directory       string
	MaxRecordBytes  int
	MaxJournalBytes int64
	Retention       int
	Now             func() time.Time
	Rename          func(string, string) error
}

// Journal serializes append-only JSONL records. An append is fsynced at each
// caller-selected security boundary; it is never read to make authorization.
type Journal struct {
	mu       sync.Mutex
	cfg      JournalConfig
	path     string
	root     *os.Root
	file     *os.File
	closed   bool
	now      func() time.Time
	rename   func(string, string) error
	sequence uint64
	lastHash string
}

func OpenJournal(cfg JournalConfig) (*Journal, error) {
	if cfg.Directory == "" {
		return nil, errors.New("journal directory is required")
	}
	if cfg.Root == nil && !filepath.IsAbs(cfg.Directory) {
		return nil, errors.New("journal directory must be absolute without a root")
	}
	if cfg.Root != nil && (!filepath.IsLocal(cfg.Directory) || cfg.Directory == ".") {
		return nil, errors.New("journal directory must be a canonical rooted relative path")
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
	if cfg.Root == nil {
		if err := secureJournalDirectory(cfg.Directory); err != nil {
			return nil, err
		}
	} else if err := secureRootedJournalDirectory(cfg.Root, cfg.Directory); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Rename == nil {
		cfg.Rename = os.Rename
	}
	path := filepath.Join(cfg.Directory, "events.jsonl")
	j := &Journal{cfg: cfg, path: path, root: cfg.Root, now: cfg.Now, rename: cfg.Rename}
	if err := j.open(); err != nil {
		return nil, err
	}
	if err := j.loadHead(); err != nil {
		_ = j.file.Close()
		return nil, err
	}
	return j, nil
}

func (j *Journal) open() error {
	if j.root != nil {
		return j.openRooted()
	}
	if info, err := os.Lstat(j.path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			return errors.New("unsafe audit journal artifact")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(j.path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
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

func secureRootedJournalDirectory(root *os.Root, directory string) error {
	dir, err := rootedOpenJournalDir(root, directory)
	if err != nil {
		return err
	}
	return dir.Close()
}

func (j *Journal) openRooted() error {
	f, err := rootedOpenJournalFile(j.root, j.path)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		_ = f.Close()
		return errors.New("unsafe opened audit journal artifact")
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
		event.Time = j.now().UTC()
	} else {
		event.Time = event.Time.UTC()
	}
	event.Metadata = sanitizeMetadata(event.Metadata)
	event.Sequence = j.sequence + 1
	event.PreviousHash = j.lastHash
	event.Hash = ""
	event.Hash = eventHash(event)
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
		if err := j.file.Sync(); err != nil {
			return err
		}
	}
	j.sequence, j.lastHash = event.Sequence, event.Hash
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
		old, next := fmt.Sprintf("%s.%d", j.path, n), fmt.Sprintf("%s.%d", j.path, n+1)
		_, err := j.lstatArtifact(old)
		if err == nil {
			if err := j.renameArtifact(old, next); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := j.renameArtifact(j.path, j.path+".1"); err != nil {
		return err
	}
	return j.open()
}

func (j *Journal) lstatArtifact(name string) (os.FileInfo, error) {
	if j.root != nil {
		return j.root.Lstat(name)
	}
	return os.Lstat(name)
}

func (j *Journal) renameArtifact(old, next string) error {
	if j.root != nil {
		return rootedRename(j.root, old, next)
	}
	return j.rename(old, next)
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
		if !auditMetadataKey[key] {
			continue
		}
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "password") || strings.Contains(lower, "header") {
			out[key] = redacted
			continue
		}
		out[key] = sanitizeMetadataValue(value)
	}
	return out
}

var auditMetadataKey = map[string]bool{
	"method": true, "kind": true, "operation": true, "environmentId": true,
	"itemId": true, "approvalId": true, "network": true, "cwd": true,
	"paths": true, "message": true,
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case string:
		value := strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' {
				return -1
			}
			return r
		}, typed)
		return redactValue(value)
	case map[string]any:
		return sanitizeMetadata(typed)
	case []any:
		out := make([]any, len(typed))
		for n, item := range typed {
			out[n] = sanitizeMetadataValue(item)
		}
		return out
	default:
		return value
	}
}

var secretValue = regexp.MustCompile(`(?i)(bearer\s+|(?:api[_-]?key|token|secret|password)\s*[=:]\s*|https?://[^\s/:]+:)[A-Za-z0-9._~+/=-]{8,}`)

func redactValue(value string) string {
	if secretValue.MatchString(value) {
		return redacted
	}
	return value
}

func eventHash(event Event) string {
	event.Hash = ""
	b, _ := json.Marshal(event)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func secureJournalDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("unsafe audit directory")
	}
	// This is only safe after Lstat has rejected a link. Existing private
	// state directories created by older versions may have a permissive mode.
	return os.Chmod(path, 0o700)
}

func (j *Journal) loadHead() error {
	info, err := j.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > j.cfg.MaxJournalBytes {
		return errors.New("audit journal exceeds configured size")
	}
	data := make([]byte, info.Size())
	if len(data) != 0 {
		if _, err := j.file.ReadAt(data, 0); err != nil {
			return err
		}
	}
	if err := recoverJournalData(data); err != nil {
		return fmt.Errorf("recover audit journal: %w", err)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		j.sequence, j.lastHash = event.Sequence, event.Hash
	}
	if j.sequence == 0 && j.root != nil {
		// A restart immediately after rotation sees an empty current segment;
		// recover the global head from the newest retained segment.
		for suffix := 1; suffix <= j.cfg.Retention; suffix++ {
			name := fmt.Sprintf("%s.%d", j.path, suffix)
			f, err := j.root.Open(name)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			segment, readErr := io.ReadAll(io.LimitReader(f, j.cfg.MaxJournalBytes))
			closeErr := f.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			for _, line := range bytes.Split(bytes.TrimSpace(segment), []byte{'\n'}) {
				var event Event
				if err := json.Unmarshal(line, &event); err != nil {
					return err
				}
				j.sequence, j.lastHash = event.Sequence, event.Hash
			}
			break
		}
	}
	return nil
}

// VerifyJournal verifies record schema and the local hash-chain anchor. A
// local chain detects accidental corruption, not a same-user rewrite of both
// the journal and its local head; callers need an exported checkpoint for that.
func VerifyJournal(path string) error {
	_, err := VerifyJournalWithAnchor(path, "")
	return err
}

// VerifyJournalWithAnchor returns the final hash after verifying the local
// chain. An optional anchor is a hash exported to a separately trusted
// location; it upgrades detection only when that location is actually trusted.
func VerifyJournalWithAnchor(path, anchor string) (string, error) {
	// Rotations retain suffixes 1..Retention. Verify oldest-to-current as one
	// sequence so deletion, substitution, or reordering of a retained segment
	// breaks the global sequence or previous-hash link.
	var data []byte
	for suffix := 32; suffix >= 1; suffix-- {
		segment, err := os.ReadFile(fmt.Sprintf("%s.%d", path, suffix))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		data = append(data, segment...)
		if len(data) != 0 && data[len(data)-1] != '\n' {
			return "", fmt.Errorf("rotated audit segment %d has incomplete tail", suffix)
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	data = append(data, current...)
	return verifyJournalData(data, anchor)
}

func verifyJournalData(data []byte, anchor string) (string, error) {
	complete := bytes.HasSuffix(data, []byte{'\n'})
	lines := bytes.Split(data, []byte{'\n'})
	if complete {
		lines = lines[:len(lines)-1]
	}
	var sequence uint64
	previous := ""
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			return "", fmt.Errorf("audit record %d is empty", index+1)
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			if index == len(lines)-1 && !complete {
				return previous, nil
			}
			return "", fmt.Errorf("audit record %d: %w", index+1, err)
		}
		if event.SchemaVersion != EventSchemaVersion || event.Instance == "" || event.Kind == "" || event.Hash == "" || event.Hash != eventHash(event) {
			return "", fmt.Errorf("audit chain breaks at record %d", index+1)
		}
		if index == 0 && (event.Sequence != 1 || event.PreviousHash != "") {
			return "", fmt.Errorf("audit chain breaks at record 1")
		}
		if index > 0 && (event.Sequence != sequence+1 || event.PreviousHash != previous) {
			return "", fmt.Errorf("audit chain breaks at record %d", index+1)
		}
		sequence, previous = event.Sequence, event.Hash
	}
	if anchor != "" && anchor != previous {
		return "", errors.New("audit external anchor does not match final hash")
	}
	return previous, nil
}

// VerifyJournalRoot verifies the fixed audit artifact beneath an already-open
// state root. The relative name is intentionally constrained to prevent a CLI
// caller from turning verification into arbitrary host-file access.
func VerifyJournalRoot(root *os.Root, relative, anchor string) (string, error) {
	if root == nil || relative != "audit/events.jsonl" {
		return "", errors.New("invalid rooted audit artifact")
	}
	var data []byte
	const maxRetainedAuditSegments = 1024
	if _, err := root.Lstat(fmt.Sprintf("%s.%d", relative, maxRetainedAuditSegments+1)); err == nil {
		return "", errors.New("audit rotation exceeds verifier safety cap")
	}
	for suffix := maxRetainedAuditSegments; suffix >= 0; suffix-- {
		name := relative
		if suffix != 0 {
			name = fmt.Sprintf("%s.%d", relative, suffix)
		}
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", errors.New("unsafe audit artifact")
		}
		file, err := root.Open(name)
		if err != nil {
			return "", err
		}
		opened, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return "", err
		}
		if !opened.Mode().IsRegular() {
			_ = file.Close()
			return "", errors.New("unsafe opened audit artifact")
		}
		segment, err := io.ReadAll(io.LimitReader(file, 16<<20))
		closeErr := file.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		data = append(data, segment...)
		if suffix != 0 && (len(data) == 0 || data[len(data)-1] != '\n') {
			return "", errors.New("rotated audit segment has incomplete tail")
		}
	}
	return verifyJournalData(data, anchor)
}

// RecoverJournal ignores one incomplete final line left by a crash. It rejects
// malformed complete records so corruption remains diagnosable.
func RecoverJournal(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return recoverJournalData(data)
}

func recoverJournalData(data []byte) error {
	complete := bytes.HasSuffix(data, []byte{'\n'})
	lines := bytes.Split(data, []byte{'\n'})
	last := len(lines) - 1
	if complete {
		last-- // Split leaves one empty item after a trailing newline.
	}
	for index := 0; index <= last; index++ {
		line := bytes.TrimSpace(lines[index])
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			if index == last && !complete {
				return nil // A crash may leave only the final record incomplete.
			}
			return fmt.Errorf("invalid audit record: %w", err)
		}
		if event.SchemaVersion != EventSchemaVersion || event.Instance == "" || event.Kind == "" {
			return errors.New("invalid audit record schema")
		}
	}
	return nil
}
