# Tools

AI calls functions to interact with system. OpenAI tool-calling protocol.

## Flow

1. Advertise tools in request
2. Model returns `tool_calls` with name + JSON args
3. Check permission (allow/ask/deny)
4. Execute or reject
5. Inject result into conversation
6. Stream final response

## Built-in

Promptline ships with safe, Go-native tools (including u-root implementations). It does not execute system binaries.

Core:
- `get_current_datetime` - RFC3339 timestamp
- `read_file` - read from disk
- `write_file` - write to disk
- `ls` - list directory (path, recursive, show_hidden). Use this for directory listing (u-root `ls`).

File operations:
- `cat` `cp` `mv` `rm` `ln` `touch` `truncate` `readlink` `realpath`

Directory operations:
- `mkdir` `pwd` `dirname` `basename`

Text processing:
- `grep` `head` `tail` `sort` `uniq` `wc` `tr` `tee` `comm` `strings` `more`

Notes:
- `grep` accepts file or directory paths. For directories, it searches regular files in that directory; set `recursive: true` to traverse subdirectories and `show_hidden: true` to include hidden entries.
- `grep` path inputs support glob patterns (for example `cmd/**/*.go` is not supported, but `cmd/*.go` and `cmd/*/main.go` are).
- Directory traversal for `grep` (and `find`) respects tool limits (max depth and max entries).

File viewing/analysis:
- `hexdump` `cmp` `md5sum` `shasum` `base64`

System information:
- `uname` `hostname` `uptime` `free` `df` `du` `ps` `pidof` `id`

Misc safe:
- `echo` `seq` `printenv` `tty` `which` `mkfifo` `mktemp` `find` `chmod` `date`

## Permissions

Default:
- **Ask**: all tools (prompt required)

Override in `config.json`:

```json
{
  "tools": {
    "allow": ["ls", "read_file"],
    "ask": ["write_file"],
    "deny": []
  }
}
```

New tools are asked by default.

## Limits and Timeouts

Defaults applied when not set in `config.json`:

```json
{
  "tool_limits": {
    "max_file_size_bytes": 10485760,
    "max_directory_depth": 8,
    "max_directory_entries": 2000
  },
  "tool_rate_limits": {
    "default_per_minute": 60,
    "per_tool": {},
    "cooldown_seconds": {}
  },
  "tool_timeouts": {
    "default_seconds": 0,
    "per_tool_seconds": {}
  }
}
```

- `default_seconds` of `0` means no default timeout; per-tool overrides still apply.

## Adding Tools

Edit `internal/tools/builtin.go` or `internal/tools/builtin_uroot.go`. Avoid `exec.Command`; tools must not shell out to system binaries.

```go
// implement
func myTool(args map[string]interface{}) (string, error) {
    param, ok := args["param"].(string)
    if !ok {
        return "", fmt.Errorf("missing param")
    }
    
    result := doStuff(param)
    return result, nil
}

// register in registerBuiltInTools()
func registerBuiltInTools(r *Registry) {
    // ... existing tools ...
    
    r.RegisterTool(&Tool{
        Name:        "my_tool",
        Description: "Does something useful",
        Parameters:  "param: string - what it does",
        Executor:    myTool,
    })
}
```

### Validation

Always validate args:

```go
func myTool(args map[string]interface{}) (string, error) {
    // required string
    name, ok := args["name"].(string)
    if !ok {
        return "", fmt.Errorf("missing name")
    }
    
    // optional with default
    count, ok := args["count"].(float64)  // JSON numbers are float64
    if !ok {
        count = 1.0
    }
    
    // boolean
    verbose, _ := args["verbose"].(bool)  // defaults to false
    
    // your logic
}
```

### Errors

Return descriptive errors, wrap with `%w`:

```go
result, err := executeQuery(query)
if err != nil {
    return "", fmt.Errorf("query failed: %w", err)
}
```

### Security

Tools that access filesystem, network, or modify data require:
- Input validation/sanitization
- Path restrictions
- Rate limiting
- User approval

Security model:
- Promptline does not execute system binaries.
- All built-in tools are implemented in Go (u-root or stdlib).

Default policy asks before running any tool unless configured otherwise.

## Structure

```
internal/tools/
├── tools.go           # registry and execution
├── builtin.go         # core tools
├── builtin_uroot.go   # u-root implementations
└── [your tool files]
```
