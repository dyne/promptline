package governance

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"promptline/internal/appserver"
)

func TestDecodeApprovalStrictlyBindsIdentityAndAuthority(t *testing.T) {
	active := ApprovalIdentity{ThreadID: "thread", TurnID: "turn"}
	valid := []byte(`{"threadId":"thread","turnId":"turn","itemId":"item","environmentId":"local","approvalId":"approval","reason":"test","command":"go test ./...","cwd":"/work","commandActions":[]}`)
	for _, tc := range []struct {
		name, method string
		params       []byte
		want         bool
	}{
		{"command", "item/commandExecution/requestApproval", valid, true},
		{"unknown method", "item/nope/requestApproval", valid, false},
		{"unknown authority field", "item/commandExecution/requestApproval", append(valid[:len(valid)-1], []byte(`,"additionalPermissions":{"network":{"enabled":true}}}`)...), false},
		{"persistent amendment", "item/commandExecution/requestApproval", append(valid[:len(valid)-1], []byte(`,"proposedExecpolicyAmendment":{}}`)...), false},
		{"network context", "item/commandExecution/requestApproval", []byte(`{"threadId":"thread","turnId":"turn","itemId":"item","networkApprovalContext":{}}`), false},
		{"wrong turn", "item/commandExecution/requestApproval", []byte(`{"threadId":"thread","turnId":"other","itemId":"item","command":"x","cwd":"/work","commandActions":[]}`), false},
		{"file without correlated diff", "item/fileChange/requestApproval", []byte(`{"threadId":"thread","turnId":"turn","itemId":"item"}`), false},
		{"permission grant", "item/permissions/requestApproval", []byte(`{"threadId":"thread","turnId":"turn","itemId":"item","cwd":"/work","permissions":{"fileSystem":{"write":["/work"]}}}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identity := active
			if tc.name == "command" {
				identity.PendingItem = []byte(`{"item":{"id":"item","threadId":"thread","turnId":"turn","type":"commandExecution","command":"go test ./...","cwd":"/work","commandActions":[]}}`)
			}
			_, err := DecodeApproval(appserver.ServerRequest{ID: 1, Method: tc.method, Params: tc.params}, identity)
			if (err == nil) != tc.want {
				t.Fatalf("DecodeApproval error = %v", err)
			}
		})
	}
}

func TestUnsupportedApprovalDeclinesOnceAndAudits(t *testing.T) {
	j, err := OpenJournal(JournalConfig{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	decision, err := HandleServerRequest(t.Context(), Policy{ActiveThreadID: "thread", ActiveTurnID: "turn"}, fixedPrompt(DecisionAccept), j, appserver.ServerRequest{ID: 5, Method: "unknown", Params: []byte(`{}`)})
	if err != nil || decision["decision"] != string(DecisionDecline) {
		t.Fatalf("decision=%v err=%v", decision, err)
	}
}

func TestTerminalPromptEscapesSpoofingAndBoundsFields(t *testing.T) {
	var output bytes.Buffer
	effect := Effect{Kind: "commandExecution", Operation: "echo \x1b]8;;https://evil\a \u202Eevil", CWD: string(bytes.Repeat([]byte("x"), terminalFieldLimit+32)), CommandActions: []byte(`[]`)}
	if _, err := (TerminalPrompt{Input: bytes.NewBufferString("no\n"), Output: &output}).Decide(effect); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	for _, unsafe := range []string{"\x1b", "\u202e"} {
		if bytes.Contains([]byte(got), []byte(unsafe)) {
			t.Fatalf("unsafe terminal sequence %q in %q", unsafe, got)
		}
	}
	if !bytes.Contains([]byte(got), []byte("[truncated]")) {
		t.Fatalf("long field was not bounded")
	}
	if errors.Is(nil, ErrUnsupportedApproval) {
		t.Fatal("impossible")
	}
}

func TestTerminalPromptRendersFilePathsAndDiffs(t *testing.T) {
	var output bytes.Buffer
	effect := Effect{Kind: "fileChange", Changes: []FileChange{{Path: "/work/a.go", Diff: "-old\n+new"}}}
	_, _ = (TerminalPrompt{Input: bytes.NewBufferString("no\n"), Output: &output}).Decide(effect)
	if got := output.String(); !bytes.Contains([]byte(got), []byte(`path="/work/a.go"`)) || !bytes.Contains([]byte(got), []byte(`diff="-old\n+new"`)) {
		t.Fatalf("incomplete file approval rendering: %q", got)
	}
}

func TestFileApprovalUsesOnlyMatchingPendingItem(t *testing.T) {
	request := appserver.ServerRequest{ID: 1, Method: "item/fileChange/requestApproval", Params: []byte(`{"threadId":"t","turnId":"u","itemId":"i"}`)}
	pending := []byte(`{"item":{"id":"i","threadId":"t","turnId":"u","type":"fileChange","changes":[{"path":"a","diff":"x"}]}}`)
	effect, err := DecodeApproval(request, ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", PendingItem: pending})
	if err != nil || len(effect.Changes) != 1 {
		t.Fatalf("effect=%#v err=%v", effect, err)
	}
	_, err = DecodeApproval(request, ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", PendingItem: []byte(`{"item":{"id":"other"}}`)})
	if !errors.Is(err, ErrUnsupportedApproval) {
		t.Fatalf("mismatch error=%v", err)
	}
	request.Params = []byte(`{"threadId":"t","turnId":"u","itemId":"i","changes":[{"path":"a","diff":"different"}]}`)
	if _, err := DecodeApproval(request, ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", PendingItem: pending}); !errors.Is(err, ErrUnsupportedApproval) {
		t.Fatalf("supplied change mismatch error=%v", err)
	}
}

func TestAvailableDecisionsCannotPermitAbsentAccept(t *testing.T) {
	got, err := Decide(Policy{}, fixedPrompt(DecisionAccept), Effect{AllowedDecisions: []Decision{DecisionDecline, DecisionCancel}})
	if err != nil || got != DecisionDecline {
		t.Fatalf("decision=%q err=%v", got, err)
	}
}

func TestCommandApprovalValidationRejectsIdentityAndDecisionDrift(t *testing.T) {
	identity := ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", EnvironmentID: "e", ApprovalID: "a"}
	valid := commandApproval{ThreadID: "t", TurnID: "u", ItemID: "i", EnvironmentID: "e", ApprovalID: "a", Command: "pwd", CWD: "/tmp", CommandActions: json.RawMessage(`[]`), AvailableDecisions: []string{"accept", "decline"}}
	if !valid.valid(identity) {
		t.Fatal("valid command rejected")
	}
	for _, mutate := range []func(*commandApproval){func(v *commandApproval) { v.ItemID = "other" }, func(v *commandApproval) { v.EnvironmentID = "other" }, func(v *commandApproval) { v.ApprovalID = "other" }, func(v *commandApproval) { v.AvailableDecisions = []string{"grant"} }, func(v *commandApproval) { v.CommandActions = json.RawMessage("bad") }} {
		candidate := valid
		mutate(&candidate)
		if candidate.valid(identity) {
			t.Fatal("drift accepted")
		}
	}
}

func TestFileAndPermissionApprovalValidationRejectsMalformedFields(t *testing.T) {
	identity := ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", EnvironmentID: "e"}
	if !(fileApproval{ThreadID: "t", TurnID: "u", ItemID: "i"}).valid(identity) {
		t.Fatal("valid file approval rejected")
	}
	if (fileApproval{ThreadID: "t", TurnID: "u", ItemID: "other"}).valid(identity) {
		t.Fatal("wrong file item accepted")
	}
	valid := permissionApproval{ThreadID: "t", TurnID: "u", ItemID: "i", EnvironmentID: "e", CWD: "/tmp", Permissions: json.RawMessage(`[]`)}
	if !valid.valid(identity) {
		t.Fatal("valid permission rejected")
	}
	valid.Permissions = json.RawMessage("bad")
	if valid.valid(identity) {
		t.Fatal("malformed permission accepted")
	}
}

func TestApprovalDecisionAndChangeComparisons(t *testing.T) {
	if got := decisions([]string{"accept", "decline"}); len(got) != 2 || got[0] != DecisionAccept {
		t.Fatalf("decisions=%v", got)
	}
	changes := []FileChange{{Path: "a", Diff: "x"}}
	if !sameChanges(changes, changes) {
		t.Fatal("same changes differ")
	}
	if sameChanges(changes, []FileChange{{Path: "a", Diff: "y"}}) || sameChanges(changes, nil) {
		t.Fatal("different changes match")
	}
	if pendingFileChanges(ApprovalIdentity{PendingItem: []byte("bad")}, fileApproval{}) != nil {
		t.Fatal("malformed pending item accepted")
	}
}

func TestCommandApprovalRequiresMatchingRenderedPendingItem(t *testing.T) {
	params := []byte(`{"threadId":"t","turnId":"u","itemId":"i","command":"go test","cwd":"/work","commandActions":[]}`)
	pending := []byte(`{"item":{"id":"i","threadId":"t","turnId":"u","type":"commandExecution","command":"go test","cwd":"/work","commandActions":[]}}`)
	request := appserver.ServerRequest{ID: 1, Method: "item/commandExecution/requestApproval", Params: params}
	if _, err := DecodeApproval(request, ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", PendingItem: pending}); err != nil {
		t.Fatalf("matching pending command: %v", err)
	}
	if _, err := DecodeApproval(request, ApprovalIdentity{ThreadID: "t", TurnID: "u", ItemID: "i", PendingItem: []byte(`{"item":{"id":"i","threadId":"t","turnId":"u","type":"commandExecution","command":"rm -rf /","cwd":"/work","commandActions":[]}}`)}); !errors.Is(err, ErrUnsupportedApproval) {
		t.Fatalf("mismatched command error=%v", err)
	}
}
