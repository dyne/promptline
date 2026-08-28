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
	instance := policy.Instance
	if instance == "" {
		instance = "runtime"
	}
	event := Event{Instance: instance, ActorUID: os.Getuid(), ActorGID: os.Getgid(), ProcessID: os.Getpid(), ThreadID: effect.ThreadID, TurnID: effect.TurnID, RequestID: requestID(request.ID), Kind: "effect-request", Metadata: auditEffect(effect, request.Method)}
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

// RecordReplyOutcome completes the durable authorization lifecycle after the
// protocol reply write has returned. It is observational only.
func RecordReplyOutcome(journal *Journal, policy Policy, request appserver.ServerRequest, decision map[string]string, replyErr error) error {
	if journal == nil {
		return nil
	}
	instance := policy.Instance
	if instance == "" {
		instance = "runtime"
	}
	outcome := "sent"
	if replyErr != nil {
		outcome = "failed"
	}
	return journal.Append(Event{Instance: instance, ActorUID: os.Getuid(), ActorGID: os.Getgid(), ProcessID: os.Getpid(), RequestID: requestID(request.ID), Kind: "effect-reply", Decision: decision["decision"], Outcome: outcome}, true)
}

func auditEffect(effect Effect, method string) map[string]any {
	metadata := map[string]any{"method": method, "kind": effect.Kind, "operation": effect.Operation, "environmentId": effect.EnvironmentID, "itemId": effect.ItemID, "approvalId": effect.ApprovalID, "network": effect.RequestsNetwork}
	if effect.CWD != "" {
		metadata["cwd"] = effect.CWD
	}
	if len(effect.Paths) != 0 {
		metadata["paths"] = effect.Paths
	}
	return metadata
}

func requestID(id uint64) string {
	return strconv.FormatUint(id, 10)
}
