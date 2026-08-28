package governance

import (
	"context"
	"os"
	"strconv"

	"promptline/internal/appserver"
)

// HandleServerRequest establishes the auditable ordering for one effect:
// request -> durable decision -> caller sends response -> caller records
// completion. If journaling fails, the returned decision is decline.
func HandleServerRequest(_ context.Context, policy Policy, prompt Prompt, journal *Journal, request appserver.ServerRequest) (map[string]string, error) {
	effect, decodeErr := DecodeApproval(request, policy.Approval)
	if decodeErr != nil {
		effect = Effect{Kind: request.Method}
	}
	event := Event{Instance: "runtime", ActorUID: os.Getuid(), ActorGID: os.Getgid(), ProcessID: os.Getpid(), RequestID: requestID(request.ID), Kind: "effect-request", Metadata: map[string]any{"method": request.Method}}
	if journal != nil {
		if err := journal.Append(event, true); err != nil {
			return map[string]string{"decision": string(DecisionDecline)}, err
		}
	}
	decision, err := Decide(policy, prompt, effect)
	if decodeErr != nil {
		decision, err = DecisionDecline, nil
	}
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
