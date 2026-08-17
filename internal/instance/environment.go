package instance

import (
	"net/url"
	"sort"
	"strings"
)

// EnvironmentPolicy explicitly describes a child environment. Parent is an
// os.Environ-style list; it is never read implicitly or mutated.
type EnvironmentPolicy struct {
	Parent    []string
	Overrides map[string]string
	Remove    []string
}

var inheritedEnvironmentKeys = map[string]bool{
	"ALL_PROXY": true, "COLORTERM": true, "CODEX_API_KEY": true, "CODEX_AUTH_TOKEN": true,
	"CODEX_CONFIG": true, "CODEX_HOME": true, "HTTP_PROXY": true, "HTTPS_PROXY": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true, "LC_MESSAGES": true, "NO_COLOR": true,
	"NO_PROXY": true, "TERM": true,
}

// ChildEnvironment builds a deterministic, allow-listed environment for an
// app-server or toolbox child. Overrides may introduce administrator-approved
// variables; Remove always wins over inherited and overridden values.
func ChildEnvironment(policy EnvironmentPolicy) []string {
	values := make(map[string]string)
	for _, entry := range policy.Parent {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !inheritedEnvironmentKeys[key] {
			continue
		}
		values[key] = value
	}
	for key, value := range policy.Overrides {
		if validEnvironmentKey(key) {
			values[key] = value
		}
	}
	for _, key := range policy.Remove {
		delete(values, key)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

// EnvironmentForChild applies the instance's private CODEX_HOME.
func (i *Instance) EnvironmentForChild(parent []string, overrides map[string]string, remove []string) []string {
	copyOverrides := make(map[string]string, len(overrides)+1)
	for key, value := range overrides {
		copyOverrides[key] = value
	}
	copyOverrides["CODEX_HOME"] = i.codexHome
	return ChildEnvironment(EnvironmentPolicy{Parent: append([]string(nil), parent...), Overrides: copyOverrides, Remove: append([]string(nil), remove...)})
}

func validEnvironmentKey(key string) bool {
	if key == "" {
		return false
	}
	for index, r := range key {
		if !(r == '_' || r >= 'A' && r <= 'Z' || index > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// RedactEnvironmentValue is the sole redaction rule for child diagnostics and
// future audit events. It redacts sensitive names and credential-bearing URLs.
func RedactEnvironmentValue(key, value string) string {
	lower := strings.ToLower(key)
	for _, sensitive := range []string{"api_key", "token", "secret", "password", "authorization", "cookie", "credential"} {
		if strings.Contains(lower, sensitive) {
			return "[REDACTED]"
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "bearer ") {
		return "[REDACTED]"
	}
	if parsed, err := url.Parse(value); err == nil && parsed.User != nil {
		return "[REDACTED]"
	}
	return value
}
