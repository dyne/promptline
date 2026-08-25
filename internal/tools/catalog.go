package tools

import "sort"

// ToolPolicyClass describes whether a built-in tool can run without an
// interactive approval in the standalone MCP server.
type ToolPolicyClass string

const (
	ToolReadOnly ToolPolicyClass = "read_only"
	ToolMutating ToolPolicyClass = "mutating"
)

// CatalogEntry is the stable semantic manifest for a built-in tool.
type CatalogEntry struct {
	Name   string
	Policy ToolPolicyClass
}

// builtInToolPolicy is deliberately complete: adding a built-in without an
// entry is a contract failure in catalog_test.go. Keep policy ownership here,
// rather than duplicating the standalone MCP allow-list.
var builtInToolPolicy = map[string]ToolPolicyClass{
	"base64": ToolReadOnly, "basename": ToolReadOnly, "cat": ToolReadOnly,
	"chmod": ToolMutating, "cmp": ToolReadOnly, "comm": ToolReadOnly,
	"cp": ToolMutating, "create_file": ToolMutating, "date": ToolReadOnly,
	"df": ToolReadOnly, "dirname": ToolReadOnly, "du": ToolReadOnly,
	"echo": ToolReadOnly, "edit_file": ToolMutating, "find": ToolReadOnly,
	"free": ToolReadOnly, "get_current_datetime": ToolReadOnly,
	"grep": ToolReadOnly, "head": ToolReadOnly, "hexdump": ToolReadOnly,
	"hostname": ToolReadOnly, "id": ToolReadOnly, "ln": ToolMutating,
	"ls": ToolReadOnly, "md5sum": ToolReadOnly, "mkdir": ToolMutating,
	"mkfifo": ToolMutating, "mktemp": ToolMutating, "more": ToolReadOnly,
	"mv": ToolMutating, "pidof": ToolReadOnly, "printenv": ToolReadOnly,
	"ps": ToolReadOnly, "pwd": ToolReadOnly, "read_file": ToolReadOnly,
	"readlink": ToolReadOnly, "realpath": ToolReadOnly, "rm": ToolMutating,
	"seq": ToolReadOnly, "shasum": ToolReadOnly, "sort": ToolReadOnly,
	"strings": ToolReadOnly, "tail": ToolReadOnly, "tee": ToolMutating,
	"touch": ToolMutating, "tr": ToolReadOnly, "truncate": ToolMutating,
	"tty": ToolReadOnly, "uname": ToolReadOnly, "uniq": ToolReadOnly,
	"uptime": ToolReadOnly, "wc": ToolReadOnly, "which": ToolReadOnly,
}

// BuiltInToolCatalog returns a deterministic copy of the built-in manifest.
func BuiltInToolCatalog() []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(builtInToolPolicy))
	for name, policy := range builtInToolPolicy {
		entries = append(entries, CatalogEntry{Name: name, Policy: policy})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// ReadOnlyToolPolicy permits only catalogued observation tools. Other tools
// retain the safe ask-by-default behavior.
func ReadOnlyToolPolicy() Policy {
	allow := make([]string, 0, len(builtInToolPolicy))
	for name, policy := range builtInToolPolicy {
		if policy == ToolReadOnly {
			allow = append(allow, name)
		}
	}
	return PolicyFromLists(allow, nil, nil)
}
