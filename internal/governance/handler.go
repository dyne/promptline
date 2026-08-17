package governance

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"promptline/internal/appserver"
)

// HandleServerRequest establishes the auditable ordering for one effect:
// request -> durable decision -> caller sends response -> caller records
// completion. If journaling fails, the returned decision is decline.
func HandleServerRequest(_ context.Context, policy Policy, prompt Prompt, journal *Journal, request appserver.ServerRequest) (map[string]string, error) {
	effect := Effect{Kind: request.Method}
	var wire struct {
		CWD    string `json:"cwd"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(request.Params, &wire)
	effect.CWD, effect.Operation = wire.CWD, wire.Reason
	event := Event{Instance: "runtime", ActorUID: os.Getuid(), ActorGID: os.Getgid(), ProcessID: os.Getpid(), RequestID: requestID(request.ID), Kind: "effect-request", Metadata: map[string]any{"method": request.Method}}
	if journal != nil {
		if err := journal.Append(event, true); err != nil {
			return map[string]string{"decision": string(DecisionDecline)}, err
		}
	}
	decision, err := Decide(policy, prompt, effect)
	if err != nil {
		decision = DecisionDecline
	}
	if journal != nil {
		event.Kind, event.Decision = "effect-decision", string(decision)
		if appendErr := journal.Append(event, true); appendErr != nil {
			return map[string]string{"decision": string(DecisionDecline)}, appendErr
		}
	}
	return map[string]string{"decision": string(decision)}, err
}

func requestID(id uint64) string {
	return strconv.FormatUint(id, 10)
}
