package governance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"promptline/internal/appserver"
)

func TestPolicyFailsClosedAndAllowsScopedRead(t *testing.T) {
	policy := Policy{Roots: []string{"/srv/work"}, AutoAllowReads: true}
	if got := policy.Evaluate(Effect{Kind: "read", Paths: []string{"/srv/work/a.txt"}}); got != DecisionAccept {
		t.Fatalf("read = %s", got)
	}
	for _, effect := range []Effect{{Kind: "write", Paths: []string{"/srv/work/a.txt"}}, {Kind: "read", Paths: []string{"/srv/workshop/a.txt"}}, {Kind: "read", Paths: []string{"/srv/work/a.txt"}, PersistentGrant: true}, {Kind: "read", Paths: []string{"/srv/work/a.txt"}, RequestsNetwork: true}} {
		if got := policy.Evaluate(effect); got == DecisionAccept {
			t.Fatalf("effect %#v was implicitly accepted", effect)
		}
	}
}

func TestDecideFailsClosedForAllTerminalBranches(t *testing.T) {
	policy := Policy{Roots: []string{"/srv/work"}, AutoAllowReads: true}
	for _, tc := range []struct {
		name    string
		effect  Effect
		prompt  Prompt
		want    Decision
		wantErr bool
	}{
		{name: "scoped read", effect: Effect{Kind: "tools/read", Paths: []string{"/srv/work/a"}}, want: DecisionAccept},
		{name: "write without prompt", effect: Effect{Kind: "write", Paths: []string{"/srv/work/a"}}, want: DecisionDecline},
		{name: "network without prompt", effect: Effect{Kind: "read", Paths: []string{"/srv/work/a"}, RequestsNetwork: true}, want: DecisionDecline},
		{name: "invalid prompt response", effect: Effect{Kind: "write"}, prompt: fixedPrompt("unknown"), want: DecisionDecline, wantErr: true},
		{name: "cancel response", effect: Effect{Kind: "write"}, prompt: fixedPrompt(DecisionCancel), want: DecisionCancel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide(policy, tc.prompt, tc.effect)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("Decide() = %q, %v; want %q, error=%t", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

type fixedPrompt Decision

func (p fixedPrompt) Decide(Effect) (Decision, error) { return Decision(p), nil }

func TestTerminalPromptFailsClosed(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Decision
	}{{"yes\n", DecisionAccept}, {"cancel\n", DecisionCancel}, {"\n", DecisionDecline}, {"unexpected\n", DecisionDecline}, {"", DecisionDecline}} {
		t.Run(test.input, func(t *testing.T) {
			var output bytes.Buffer
			got, err := (TerminalPrompt{Input: bytes.NewBufferString(test.input), Output: &output}).Decide(Effect{Kind: "command", Operation: "rm", CWD: "/tmp"})
			if err != nil || got != test.want {
				t.Fatalf("decision=%s err=%v", got, err)
			}
		})
	}
}

func TestHandleServerRequestAuditsAndDeclinesWithoutTerminal(t *testing.T) {
	journal, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	decision, err := HandleServerRequest(context.Background(), Policy{}, nil, journal, appserver.ServerRequest{ID: 4, Method: "item/commandExecution/requestApproval"})
	if err != nil {
		t.Fatal(err)
	}
	if decision["decision"] != string(DecisionDecline) {
		t.Fatalf("decision=%v", decision)
	}
}

func TestAuditLifecycleIncludesIdentityAndReplyOutcome(t *testing.T) {
	dir := t.TempDir()
	journal, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	request := appserver.ServerRequest{ID: 9, Method: "item/commandExecution/requestApproval"}
	policy := Policy{Instance: "instance-a"}
	decision, err := HandleServerRequest(context.Background(), policy, nil, journal, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordReplyOutcome(journal, policy, request, decision, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"instance":"instance-a"`)) || !bytes.Contains(data, []byte(`"kind":"effect-reply"`)) || !bytes.Contains(data, []byte(`"outcome":"sent"`)) {
		t.Fatalf("incomplete audit lifecycle: %s", data)
	}
	if err := RecordReplyOutcome(journal, policy, request, decision, errors.New("write failed")); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"outcome":"failed"`)) {
		t.Fatalf("reply failure was not durably recorded: %s", data)
	}
}

func TestJournalRedactsRotatesAndUsesPrivateModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "journal")
	journal, err := OpenJournal(JournalConfig{Directory: dir, MaxJournalBytes: 180, Retention: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = journal.Close() })
	for n := 0; n < 3; n++ {
		err = journal.Append(Event{Instance: "test", Kind: "approval", Time: time.Unix(int64(n), 0), Metadata: map[string]any{"apiToken": "secret", "message": "ok\nnext"}}, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data) == "secret" {
		t.Fatalf("journal did not write/redact: %q", data)
	}
	if contains(string(data), "secret") {
		t.Fatalf("secret was retained: %s", data)
	}
	info, err := os.Stat(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := RecoverJournal(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func contains(text, want string) bool {
	return len(want) == 0 || (len(text) >= len(want) && index(text, want) >= 0)
}
func index(text, want string) int {
	for n := 0; n+len(want) <= len(text); n++ {
		if text[n:n+len(want)] == want {
			return n
		}
	}
	return -1
}

func TestJournalRotationPropagatesInjectedRenameFault(t *testing.T) {
	fault := errors.New("rename fault")
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir(), MaxJournalBytes: 1, Rename: func(string, string) error { return fault }})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	err = j.Append(Event{Instance: "test", Kind: "effect"}, true)
	if !errors.Is(err, fault) {
		t.Fatalf("Append error = %v", err)
	}
}

func TestJournalRecoveryAndNestedRedaction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	valid := []byte(`{"schemaVersion":1,"time":"2026-01-01T00:00:00Z","instance":"test","kind":"approval"}`)
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "complete record", data: append(append([]byte{}, valid...), '\n')},
		{name: "crash tail", data: append(append(append([]byte{}, valid...), '\n'), []byte(`{"schemaVersion":`)...)},
		{name: "malformed complete record", data: append(append(append([]byte{}, valid...), '\n'), append([]byte(`{"schemaVersion":}`), '\n')...), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := RecoverJournal(path); (err != nil) != tc.wantErr {
				t.Fatalf("RecoverJournal() error = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if err := j.Append(Event{Instance: "test", Kind: "approval", Metadata: map[string]any{"nested": map[string]any{"apiToken": "secret", "note": "ok\nnext"}, "items": []any{map[string]any{"password": "secret"}}}}, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(data), "secret") {
		t.Fatalf("nested sensitive metadata leaked: %s", data)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	var event Event
	if err := json.Unmarshal(lines[len(lines)-1], &event); err != nil {
		t.Fatal(err)
	}
}

func TestJournalRejectsSymlinkArtifactsAndDetectsTampering(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(outside, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(JournalConfig{Directory: dir}); err == nil {
		t.Fatal("journal symlink accepted")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "unchanged" {
		t.Fatalf("outside sentinel = %q, %v", got, err)
	}
	if err := os.Remove(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Event{Instance: "test", Kind: "effect"}, true); err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Event{Instance: "test", Kind: "reply"}, true); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.jsonl")
	if err := VerifyJournal(path); err != nil {
		t.Fatalf("verify intact journal: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"kind":"reply"`), []byte(`"kind":"alter"`), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyJournal(path); err == nil {
		t.Fatal("tampered journal verified")
	}
}

func TestJournalUsesProvidedRootForRelativeArtifacts(t *testing.T) {
	base := t.TempDir()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	j, err := OpenJournal(JournalConfig{Root: root, Directory: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if err := j.Append(Event{Instance: "test", Kind: "effect"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Lstat("audit/events.jsonl"); err != nil {
		t.Fatalf("rooted journal missing: %v", err)
	}
	if _, err := OpenJournal(JournalConfig{Root: root, Directory: "../escape"}); err == nil {
		t.Fatal("rooted traversal accepted")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Remove("audit/events.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(base, "audit", "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(JournalConfig{Root: root, Directory: "audit"}); err == nil {
		t.Fatal("rooted leaf symlink accepted")
	}
}

func TestJournalRedactsSensitiveValuesUnderBenignKeys(t *testing.T) {
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, secret := range []string{"Bearer abcdefghijkl", "https://alice:abcdefgh@example.test", "API_KEY=abcdefghijk"} {
		if err := j.Append(Event{Instance: "test", Kind: "effect", Metadata: map[string]any{"note": secret}}, true); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(j.cfg.Directory, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"abcdefghijkl", "alice:"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("secret leaked: %s", secret)
		}
	}
}

func TestJournalDropsUnapprovedMetadata(t *testing.T) {
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if err := j.Append(Event{Instance: "test", Kind: "effect", Metadata: map[string]any{"modelOutput": "do not retain", "itemId": "safe"}}, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(j.cfg.Directory, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("modelOutput")) || !bytes.Contains(data, []byte("itemId")) {
		t.Fatalf("metadata allow-list violated: %s", data)
	}
}

func TestJournalConcurrentAppendAndClose(t *testing.T) {
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for n := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = j.Append(Event{Instance: "test", Kind: "effect", RequestID: "race"}, true)
		}()
		_ = n
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err := j.Append(Event{Instance: "test", Kind: "effect"}, true); err == nil {
		t.Fatal("Append after Close unexpectedly succeeded")
	}
}
