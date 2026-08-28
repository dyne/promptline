package governance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"promptline/internal/appserver"
)

type ApprovalIdentity struct {
	ThreadID, TurnID, ItemID, EnvironmentID, ApprovalID string
	PendingItem                                         json.RawMessage
}

var ErrUnsupportedApproval = errors.New("unsupported or malformed approval request")

// DecodeApproval admits only request shapes Promptline can display completely.
func DecodeApproval(request appserver.ServerRequest, active ApprovalIdentity) (Effect, error) {
	if request.ID == 0 {
		return Effect{}, ErrUnsupportedApproval
	}
	switch request.Method {
	case "item/commandExecution/requestApproval":
		var w commandApproval
		if err := strictDecode(request.Params, &w); err != nil || !w.valid(active) || !pendingCommandMatches(active, w) {
			return Effect{}, ErrUnsupportedApproval
		}
		return Effect{Kind: "commandExecution", Operation: w.Command, CWD: w.CWD, ThreadID: w.ThreadID, TurnID: w.TurnID, ItemID: w.ItemID, EnvironmentID: w.EnvironmentID, ApprovalID: w.ApprovalID, CommandActions: w.CommandActions, AllowedDecisions: decisions(w.AvailableDecisions)}, nil
	case "item/fileChange/requestApproval":
		var w fileApproval
		if err := strictDecode(request.Params, &w); err != nil || !w.valid(active) {
			return Effect{}, ErrUnsupportedApproval
		}
		changes := pendingFileChanges(active, w)
		if len(changes) == 0 || (len(w.Changes) != 0 && !sameChanges(w.Changes, changes)) {
			return Effect{}, ErrUnsupportedApproval
		}
		return Effect{Kind: "fileChange", Operation: w.Reason, ThreadID: w.ThreadID, TurnID: w.TurnID, ItemID: w.ItemID, Changes: changes, AllowedDecisions: decisions(w.AvailableDecisions)}, nil
	case "item/permissions/requestApproval":
		var w permissionApproval
		if err := strictDecode(request.Params, &w); err != nil || !w.valid(active) {
			return Effect{}, ErrUnsupportedApproval
		}
		// Permission profiles can add filesystem/network authority; no explicit
		// grant policy exists, so a simple approval cannot grant them.
		return Effect{}, ErrUnsupportedApproval
	default:
		return Effect{}, ErrUnsupportedApproval
	}
}
func strictDecode(data []byte, into any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(into); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

type commandApproval struct {
	ThreadID           string          `json:"threadId"`
	TurnID             string          `json:"turnId"`
	ItemID             string          `json:"itemId"`
	EnvironmentID      string          `json:"environmentId"`
	ApprovalID         string          `json:"approvalId"`
	Reason             string          `json:"reason"`
	Command            string          `json:"command"`
	CWD                string          `json:"cwd"`
	CommandActions     json.RawMessage `json:"commandActions"`
	AvailableDecisions []string        `json:"availableDecisions"`
}

func (w commandApproval) valid(a ApprovalIdentity) bool {
	if w.ThreadID == "" || w.TurnID == "" || w.ItemID == "" || w.Command == "" || w.CWD == "" || len(w.CommandActions) == 0 || w.ThreadID != a.ThreadID || w.TurnID != a.TurnID || !json.Valid(w.CommandActions) {
		return false
	}
	if a.ItemID != "" && w.ItemID != a.ItemID {
		return false
	}
	if a.EnvironmentID != "" && w.EnvironmentID != a.EnvironmentID {
		return false
	}
	if a.ApprovalID != "" && w.ApprovalID != a.ApprovalID {
		return false
	}
	for _, d := range w.AvailableDecisions {
		if d != "accept" && d != "decline" && d != "cancel" {
			return false
		}
	}
	return true
}

// pendingCommandMatches binds an approval to the command item that the
// terminal already rendered. Request fields alone are attacker-controlled.
func pendingCommandMatches(a ApprovalIdentity, request commandApproval) bool {
	var pending struct {
		Item struct {
			ID             string          `json:"id"`
			ThreadID       string          `json:"threadId"`
			TurnID         string          `json:"turnId"`
			Type           string          `json:"type"`
			Command        string          `json:"command"`
			CWD            string          `json:"cwd"`
			CommandActions json.RawMessage `json:"commandActions"`
			EnvironmentID  string          `json:"environmentId"`
			ApprovalID     string          `json:"approvalId"`
		} `json:"item"`
	}
	if json.Unmarshal(a.PendingItem, &pending) != nil {
		return false
	}
	p := pending.Item
	if p.ID != request.ItemID || p.ThreadID != request.ThreadID || p.TurnID != request.TurnID || p.Type != "commandExecution" || p.Command != request.Command || p.CWD != request.CWD || !bytes.Equal(p.CommandActions, request.CommandActions) {
		return false
	}
	return (p.EnvironmentID == "" || p.EnvironmentID == request.EnvironmentID) && (p.ApprovalID == "" || p.ApprovalID == request.ApprovalID)
}

type fileApproval struct {
	ThreadID           string       `json:"threadId"`
	TurnID             string       `json:"turnId"`
	ItemID             string       `json:"itemId"`
	Reason             string       `json:"reason"`
	Changes            []FileChange `json:"changes"`
	AvailableDecisions []string     `json:"availableDecisions"`
}

func decisions(values []string) []Decision {
	out := make([]Decision, 0, len(values))
	for _, value := range values {
		out = append(out, Decision(value))
	}
	return out
}
func pendingFileChanges(a ApprovalIdentity, request fileApproval) []FileChange {
	var item struct {
		Item struct {
			ID       string       `json:"id"`
			ThreadID string       `json:"threadId"`
			TurnID   string       `json:"turnId"`
			Type     string       `json:"type"`
			Changes  []FileChange `json:"changes"`
		} `json:"item"`
	}
	if json.Unmarshal(a.PendingItem, &item) != nil || item.Item.ID != request.ItemID || item.Item.ThreadID != request.ThreadID || item.Item.TurnID != request.TurnID || item.Item.Type != "fileChange" {
		return nil
	}
	return item.Item.Changes
}

func sameChanges(left, right []FileChange) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (w fileApproval) valid(a ApprovalIdentity) bool {
	return w.ThreadID != "" && w.TurnID != "" && w.ItemID != "" && w.ThreadID == a.ThreadID && w.TurnID == a.TurnID && (a.ItemID == "" || w.ItemID == a.ItemID)
}

type permissionApproval struct {
	ThreadID      string          `json:"threadId"`
	TurnID        string          `json:"turnId"`
	ItemID        string          `json:"itemId"`
	EnvironmentID string          `json:"environmentId"`
	CWD           string          `json:"cwd"`
	Reason        string          `json:"reason"`
	Permissions   json.RawMessage `json:"permissions"`
}

func (w permissionApproval) valid(a ApprovalIdentity) bool {
	return w.ThreadID != "" && w.TurnID != "" && w.ItemID != "" && w.CWD != "" && len(w.Permissions) != 0 && json.Valid(w.Permissions) && w.ThreadID == a.ThreadID && w.TurnID == a.TurnID && (a.EnvironmentID == "" || w.EnvironmentID == a.EnvironmentID)
}

type FileChange struct {
	Path string `json:"path"`
	Diff string `json:"diff"`
}

func (e Effect) ApprovalSummary() string {
	if e.Kind == "commandExecution" {
		return fmt.Sprintf("command=%q\ncwd=%q\nactions=%s", e.Operation, e.CWD, e.CommandActions)
	}
	if e.Kind == "fileChange" {
		var b bytes.Buffer
		fmt.Fprintf(&b, "file changes=%d", len(e.Changes))
		for _, change := range e.Changes {
			fmt.Fprintf(&b, "\npath=%q\ndiff=%q", change.Path, change.Diff)
		}
		return b.String()
	}
	return ""
}
