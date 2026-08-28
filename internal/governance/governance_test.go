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
	if _, err := VerifyJournalRoot(root, "audit/events.jsonl", ""); err != nil {
		t.Fatalf("verify rooted journal: %v", err)
	}
	if _, err := VerifyJournalRoot(root, "other/events.jsonl", ""); err == nil {
		t.Fatal("arbitrary rooted artifact accepted")
	}
	if _, err := VerifyJournalRoot(nil, "audit/events.jsonl", ""); err == nil {
		t.Fatal("nil root accepted")
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

func TestVerifyJournalDataRejectsBrokenLifecycleShapes(t *testing.T) {
	event := Event{SchemaVersion: EventSchemaVersion, Instance: "test", Kind: "effect", Sequence: 1}
	event.Hash = eventHash(event)
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyJournalData(append(encoded, '\n'), event.Hash); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, []byte("\n"), []byte("not-json\n")} {
		if _, err := verifyJournalData(data, ""); err == nil {
			t.Fatalf("invalid data accepted: %q", data)
		}
	}
	if _, err := verifyJournalData(append(encoded, '\n'), "wrong"); err == nil {
		t.Fatal("wrong anchor accepted")
	}
}

func TestAuditMetadataSanitizesNestedTypesAndControls(t *testing.T) {
	metadata := sanitizeMetadata(map[string]any{
		"token": "visible", "unknown": "drop", "message": []any{"ok\x00", map[string]any{"password": "nope"}}, "paths": 7,
	})
	if metadata["token"] != nil || metadata["paths"] != 7 || metadata["unknown"] != nil {
		t.Fatalf("metadata policy = %#v", metadata)
	}
	values := metadata["message"].([]any)
	if values[0] != "ok" || len(values[1].(map[string]any)) != 0 {
		t.Fatalf("nested metadata = %#v", values)
	}
}

func TestRootedAuditOperationsRejectTraversalAndSupportRename(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := rootedOpenJournalDir(root, "audit"); err != nil {
		t.Fatal(err)
	}
	file, err := rootedOpenJournalFile(root, "audit/one")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rootedRename(root, "audit/one", "audit/two"); err != nil {
		t.Fatal(err)
	}
	if _, err := rootedOpenJournalFile(root, "../escape"); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := rootedRename(root, "audit/two", "../escape"); err == nil {
		t.Fatal("rename traversal accepted")
	}
}

func TestJournalRotationAndAnchorVerification(t *testing.T) {
	dir := t.TempDir()
	j, err := OpenJournal(JournalConfig{Directory: dir, MaxJournalBytes: 220, Retention: 10})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := j.Append(Event{Instance: "test", Kind: "effect", Metadata: map[string]any{"message": "rotation payload"}}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.jsonl")
	hash, err := VerifyJournalWithAnchor(path, "")
	if err != nil || hash == "" {
		t.Fatalf("verify rotation: %q %v", hash, err)
	}
	if _, err := VerifyJournalWithAnchor(path, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJournalWithAnchor(path, "wrong"); err == nil {
		t.Fatal("wrong rotated anchor accepted")
	}
}

func TestJournalRejectsInvalidConfigurationAndAppendBoundaries(t *testing.T) {
	if _, err := OpenJournal(JournalConfig{}); err == nil {
		t.Fatal("empty journal config accepted")
	}
	if _, err := OpenJournal(JournalConfig{Directory: "relative"}); err == nil {
		t.Fatal("relative journal accepted")
	}
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir(), MaxRecordBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Event{}, false); err == nil {
		t.Fatal("invalid event accepted")
	}
	if err := j.Append(Event{Instance: "x", Kind: "x", Metadata: map[string]any{"message": "too long"}}, false); err == nil {
		t.Fatal("oversized record accepted")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Event{Instance: "x", Kind: "x"}, false); err == nil {
		t.Fatal("closed journal accepted append")
	}
}

func TestRecoverJournalHandlesMissingCrashTailAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := RecoverJournal(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"bad":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverJournal(path); err != nil {
		t.Fatalf("crash tail: %v", err)
	}
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RecoverJournal(path); err == nil {
		t.Fatal("complete corruption accepted")
	}
}

func TestVerifyJournalRootRejectsUnsafeAndIncompleteArtifacts(t *testing.T) {
	base := t.TempDir()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Mkdir("audit", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "audit", "events.jsonl"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJournalRoot(root, "audit/events.jsonl", ""); err == nil {
		t.Fatal("corrupt rooted journal accepted")
	}
	if err := os.WriteFile(filepath.Join(base, "audit", "events.jsonl.1"), []byte("tail"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJournalRoot(root, "audit/events.jsonl", ""); err == nil {
		t.Fatal("incomplete rotated segment accepted")
	}
	if err := os.Remove(filepath.Join(base, "audit", "events.jsonl.1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(base, "audit", "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(base, "audit", "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJournalRoot(root, "audit/events.jsonl", ""); err == nil {
		t.Fatal("symlink journal accepted")
	}
}

func TestOpenJournalRejectsInvalidPersistedHead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(JournalConfig{Directory: dir}); err == nil {
		t.Fatal("corrupt journal head accepted")
	}
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"instance":"x","kind":"effect","sequence":2,"hash":"bad"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	j, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatalf("recoverable single persisted head rejected: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
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

func TestRecordReplyOutcomeRecordsSentAndFailed(t *testing.T) {
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	request := appserver.ServerRequest{ID: 7}
	if err := RecordReplyOutcome(j, Policy{}, request, map[string]string{"decision": "accept"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := RecordReplyOutcome(j, Policy{Instance: "test"}, request, map[string]string{"decision": "decline"}, errors.New("reply")); err != nil {
		t.Fatal(err)
	}
	if err := RecordReplyOutcome(nil, Policy{}, request, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAuditEffectIncludesOptionalAuthorityFields(t *testing.T) {
	metadata := auditEffect(Effect{Kind: "write", Operation: "edit", CWD: "/work", Paths: []string{"a"}, EnvironmentID: "env", ItemID: "item", ApprovalID: "approval", RequestsNetwork: true}, "method")
	if metadata["cwd"] != "/work" || metadata["network"] != true || len(metadata["paths"].([]string)) != 1 {
		t.Fatalf("audit metadata=%#v", metadata)
	}
}

func TestVerifyJournalWithAnchorRejectsMissingAndBrokenRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if _, err := VerifyJournalWithAnchor(path, ""); err == nil {
		t.Fatal("missing journal accepted")
	}
	if err := os.WriteFile(path+".1", []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyJournalWithAnchor(path, ""); err == nil {
		t.Fatal("partial rotation accepted")
	}
}

func TestVerifyJournalRejectsCrashTailEvenWithAnchor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	j, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append(Event{Instance: "test", Kind: "effect"}, true); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := VerifyJournalWithAnchor(path, "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"schemaVersion":`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, anchor := range []string{"", hash} {
		if _, err := VerifyJournalWithAnchor(path, anchor); err == nil {
			t.Fatalf("incomplete verification tail accepted with anchor %q", anchor)
		}
	}
}

func TestRootedLoadHeadRecoversRetainedRotation(t *testing.T) {
	base := t.TempDir()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Mkdir("audit", 0o700); err != nil {
		t.Fatal(err)
	}
	event := Event{SchemaVersion: EventSchemaVersion, Instance: "test", Kind: "effect", Sequence: 1}
	event.Hash = eventHash(event)
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "audit", "events.jsonl.1"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile("audit/events.jsonl", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	j := &Journal{root: root, path: "audit/events.jsonl", file: file, cfg: JournalConfig{MaxJournalBytes: 1024, Retention: 2}}
	if err := j.loadHead(); err != nil {
		t.Fatal(err)
	}
	if j.sequence != 1 || j.lastHash != event.Hash {
		t.Fatalf("head=%d %q", j.sequence, j.lastHash)
	}
	_ = file.Close()
	oversized, err := os.OpenFile(filepath.Join(base, "audit", "events.jsonl.1"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	limited := &Journal{file: oversized, cfg: JournalConfig{MaxJournalBytes: 1}}
	if err := limited.loadHead(); err == nil {
		t.Fatal("oversized journal accepted")
	}
	_ = oversized.Close()
}

type failingAuditWriter struct{}

func (failingAuditWriter) Write([]byte) (int, error) { return 0, errors.New("terminal write failed") }

func TestTerminalPromptAndJournalArtifactErrorsFailClosed(t *testing.T) {
	if decision, err := (TerminalPrompt{Input: bytes.NewBufferString("yes\n"), Output: failingAuditWriter{}}).Decide(Effect{Kind: "write"}); err == nil || decision != DecisionDecline {
		t.Fatalf("terminal write failure = %q, %v", decision, err)
	}
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if _, err := j.lstatArtifact(filepath.Join(j.cfg.Directory, "missing")); err == nil {
		t.Fatal("missing artifact accepted")
	}
	if err := j.renameArtifact(filepath.Join(j.cfg.Directory, "missing"), filepath.Join(j.cfg.Directory, "next")); err == nil {
		t.Fatal("missing rename accepted")
	}
}

func TestLoadHeadRestoresSequenceFromMultiplePersistedEvents(t *testing.T) {
	dir := t.TempDir()
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
	reopened, err := OpenJournal(JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.sequence != 2 || reopened.lastHash == "" {
		t.Fatalf("head=%d %q", reopened.sequence, reopened.lastHash)
	}
}

func TestRootedJournalOpenRejectsUnsafeDirectoryAndLeaf(t *testing.T) {
	base := t.TempDir()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(base, "audit"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := rootedOpenJournalDir(root, "audit"); err == nil {
		t.Fatal("regular audit path accepted")
	}
	if err := os.Remove(filepath.Join(base, "audit")); err != nil {
		t.Fatal(err)
	}
	if _, err := rootedOpenJournalDir(root, "audit"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(base, "audit", "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := rootedOpenJournalFile(root, "audit/events.jsonl"); err == nil {
		t.Fatal("symlink leaf accepted")
	}
}
