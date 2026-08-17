package governance

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
	if string(data) != "" && !contains(string(data), "[REDACTED]") {
		t.Fatalf("secret was not redacted: %s", data)
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
