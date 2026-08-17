// Package governance contains Promptline's provider-neutral effect policy and
// append-only audit journal. It deliberately does not authorize by audit data.
package governance

import (
	"errors"
	"path/filepath"
	"strings"
)

type Decision string

const (
	DecisionAccept  Decision = "accept"
	DecisionDecline Decision = "decline"
	DecisionCancel  Decision = "cancel"
)

type Effect struct {
	Kind              string
	Operation         string
	CWD               string
	Paths             []string
	RequestsNetwork   bool
	PersistentGrant   bool
	PrivilegeExpanded bool
}

// Policy defaults to ask for all effects. Read effects can be automatically
// accepted only when they remain beneath an explicitly configured root.
type Policy struct {
	Roots          []string
	AutoAllowReads bool
}

func (p Policy) Evaluate(effect Effect) Decision {
	if effect.PersistentGrant || effect.PrivilegeExpanded || effect.RequestsNetwork {
		return DecisionCancel // an explicit terminal confirmation is required
	}
	if !p.AutoAllowReads || !isRead(effect.Kind) || len(effect.Paths) == 0 {
		return DecisionCancel
	}
	for _, path := range effect.Paths {
		if !p.contains(path) {
			return DecisionCancel
		}
	}
	return DecisionAccept
}

func isRead(kind string) bool {
	return kind == "read" || kind == "tools/read" || strings.HasSuffix(kind, "/read")
}

func (p Policy) contains(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	for _, root := range p.Roots {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// Prompt obtains an explicit terminal decision. Callers must treat missing or
// malformed terminal input as decline/cancel, never as an implicit approval.
type Prompt interface {
	Decide(Effect) (Decision, error)
}

func Decide(policy Policy, prompt Prompt, effect Effect) (Decision, error) {
	if decision := policy.Evaluate(effect); decision == DecisionAccept {
		return decision, nil
	}
	if prompt == nil {
		return DecisionDecline, nil
	}
	decision, err := prompt.Decide(effect)
	if err != nil {
		return DecisionDecline, err
	}
	switch decision {
	case DecisionAccept, DecisionDecline, DecisionCancel:
		return decision, nil
	default:
		return DecisionDecline, errors.New("invalid approval decision")
	}
}
