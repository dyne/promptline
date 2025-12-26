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

- `get_current_datetime` - RFC3339 timestamp
- `ls` - list directory (path, recursive, show_hidden)
- `read_file` - read from disk
- `write_file` - write to disk
- `execute_shell_command` - run shell command

## Permissions

Default:
- **Ask**: all tools (prompt required)

Override in `config.json`:

```json
{
  "tools": {
    "allow": ["ls", "read_file"],
    "ask": ["write_file"],
    "deny": ["execute_shell_command"]
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
    "cooldown_seconds": {
      "execute_shell_command": 2
    }
  },
  "tool_timeouts": {
    "default_seconds": 0,
    "per_tool_seconds": {
      "execute_shell_command": 5
    }
  }
}
```

- `default_seconds` of `0` means no default timeout; per-tool overrides still apply.

## Adding Tools

Edit `internal/tools/builtin.go`:

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

Tools that execute commands, access filesystem, network, modify data need:
- Input validation/sanitization
- Path restrictions
- Command allowlists
- Rate limiting
- User approval

Default policy asks before running any tool unless configured otherwise.

## Example

```go
// list_directory tool
func listDirectory(args map[string]interface{}) (string, error) {
    path, ok := args["path"].(string)
    if !ok || path == "" {
        path = "."
    }
    
    cmd := exec.Command("ls", "-lh", path)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to list '%s': %v", path, err)
    }
    
    return string(output), nil
}

// register
r.RegisterTool(&Tool{
    Name:        "list_directory",
    Description: "List files in directory",
    Parameters:  "path: string - directory path (default: current)",
    Executor:    listDirectory,
})
```

## Structure

```
internal/tools/
├── tools.go         # framework (don't touch)
│   ├── ExecutorFunc
│   ├── Tool struct
│   ├── Registry
│   └── parsing/execution
│
└── builtin.go       # implementations (add here)
    ├── registerBuiltInTools()
    ├── getCurrentDatetime()
    ├── executeShellCommand()
    ├── readFile()
    ├── writeFile()
    └── [your tool]()
```
