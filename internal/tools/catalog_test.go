package tools

import (
	"regexp"
	"testing"
)

func TestBuiltInToolCatalogMatchesRegistry(t *testing.T) {
	registry := NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	seen := make(map[string]bool)
	validName := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, tool := range registry.GetTools() {
		name := tool.Name()
		if !validName.MatchString(name) {
			t.Errorf("invalid tool name %q", name)
		}
		if seen[name] {
			t.Errorf("duplicate registered tool %q", name)
		}
		seen[name] = true
		if tool.Description() == "" {
			t.Errorf("%s has an empty description", name)
		}
		if schema := tool.Parameters(); schema == nil || schema["type"] != "object" {
			t.Errorf("%s schema = %#v, want object schema", name, schema)
		}
		if _, ok := builtInToolPolicy[name]; !ok {
			t.Errorf("%s is not classified in the built-in policy catalog", name)
		}
	}
	for _, entry := range BuiltInToolCatalog() {
		if !seen[entry.Name] {
			t.Errorf("catalogued %s is not registered", entry.Name)
		}
		if entry.Policy != ToolReadOnly && entry.Policy != ToolMutating {
			t.Errorf("%s has invalid policy %q", entry.Name, entry.Policy)
		}
	}
	if len(seen) != len(BuiltInToolCatalog()) {
		t.Errorf("registered tools = %d, catalog = %d", len(seen), len(BuiltInToolCatalog()))
	}
}

func TestReadOnlyToolPolicyMatchesCatalog(t *testing.T) {
	policy := ReadOnlyToolPolicy()
	for _, entry := range BuiltInToolCatalog() {
		got := policy.Allow[entry.Name]
		if got != (entry.Policy == ToolReadOnly) {
			t.Errorf("%s allow = %t, policy = %s", entry.Name, got, entry.Policy)
		}
	}
}
