# Adding New Tools to Promptline

This guide explains how to add new tools to Promptline's function calling system.

## Architecture Overview

The tool system has a clean, maintainable architecture:

```
internal/tools/
├── tools.go      # Core types and registry (DO NOT modify tool implementations here)
└── builtin.go    # Built-in tool implementations (ADD new tools here)
```

### Key Components

1. **ExecutorFunc**: Function signature `func(args map[string]interface{}) (string, error)`
2. **Tool**: Struct containing name, description, parameters, and executor function
3. **Registry**: Container that manages all tools and executes them
4. **ToolCall**: Parsed function call from AI (function name + arguments)
5. **ToolResult**: Result of tool execution (success or error)

## How to Add a New Tool

### Step 1: Implement Your Tool Function

Add your implementation to `internal/tools/builtin.go`:

```go
func myNewTool(args map[string]interface{}) (string, error) {
	// Extract and validate parameters
	param1, ok := args["param1"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'param1' parameter")
	}
	
	param2, ok := args["param2"].(int)
	if !ok {
		// Handle type conversion if needed
		param2Float, ok := args["param2"].(float64)
		if !ok {
			return "", fmt.Errorf("missing or invalid 'param2' parameter")
		}
		param2 = int(param2Float)
	}
	
	// Implement your tool logic
	result := fmt.Sprintf("Processed %s with %d", param1, param2)
	
	return result, nil
}
```

### Step 2: Register Your Tool

Add registration in the `registerBuiltInTools()` function in `builtin.go`:

```go
func registerBuiltInTools(r *Registry) {
	// ... existing tools ...
	
	r.RegisterTool(&Tool{
		Name:        "my_new_tool",
		Description: "Clear description of what your tool does",
		Parameters:  "param1: string - First parameter, param2: int - Second parameter",
		Executor:    myNewTool,
	})
}
```

### Step 3: Test Your Tool

Build and run:
```bash
make build
./promptline
```

Then ask the AI to use your tool:
```
User: Can you use my_new_tool with "hello" and 42?

Assistant: I'll call my_new_tool.
Tool call: my_new_tool {"param1":"hello","param2":42}
Tool result (TOON): {result:"Processed hello with 42"}
Assistant: The tool executed successfully and returned: Processed hello with 42
```

## Best Practices

### 1. Parameter Validation

Always validate and handle type conversions:

```go
func myTool(args map[string]interface{}) (string, error) {
	// Required string parameter
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'name' parameter")
	}
	
	// Optional parameter with default
	count, ok := args["count"].(float64)  // JSON numbers are float64
	if !ok {
		count = 1.0  // Default value
	}
	
	// Boolean parameter
	verbose, _ := args["verbose"].(bool)  // Defaults to false if missing
	
	// ... your logic ...
}
```

### 2. Error Handling

Return descriptive errors:

```go
func readDatabase(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("missing 'query' parameter")
	}
	
	result, err := executeQuery(query)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)  // Wrap errors
	}
	
	return result, nil
}
```

### 3. Security Considerations

Be careful with tools that:
- Execute system commands
- Access the filesystem
- Make network requests
- Modify data

Consider adding:
- Input validation and sanitization
- Path restrictions (e.g., only allow relative paths)
- Command allowlists
- Rate limiting
- User confirmation for dangerous operations

Promptline blocks new tools by default and asks for confirmation unless you explicitly allow them via the `tools.allow`/`tools.require_confirmation` settings in `config.json`. Keep that in mind when testing new additions.

### 4. Documentation

Write clear descriptions:

```go
r.RegisterTool(&Tool{
	Name:        "search_files",
	Description: "Search for files matching a pattern in a directory. Returns list of matching file paths.",
	Parameters:  "directory: string - Directory to search in, pattern: string - Glob pattern (e.g., '*.go')",
	Executor:    searchFiles,
})
```

## Complete Example: Adding a "list_directory" Tool

```go
// In builtin.go

// Add to registerBuiltInTools()
func registerBuiltInTools(r *Registry) {
	// ... existing tools ...
	
	r.RegisterTool(&Tool{
		Name:        "list_directory",
		Description: "List files and directories in a given path",
		Parameters:  "path: string - Directory path to list (default: current directory)",
		Executor:    listDirectory,
	})
}

// Implementation
func listDirectory(args map[string]interface{}) (string, error) {
	// Get path, default to current directory
	path, ok := args["path"].(string)
	if !ok || path == "" {
		path = "."
	}
	
	// Execute ls command
	cmd := exec.Command("ls", "-lh", path)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list directory '%s': %v", path, err)
	}
	
	return string(output), nil
}
```

## Advanced: Tool Chaining

Tools can be chained - the AI can call multiple tools in sequence:

```
User: Find all Go files and show me the largest one

Assistant:
Tool call: execute_shell_command {"command":"find . -name '*.go' -exec ls -lh {} + | sort -k5 -hr | head -1"}
Tool result (TOON): {result:"-rw-r--r-- 1 user user 12K Dec  9 22:50 ./cmd/promptline/main.go"}
Assistant: The largest Go file is ./cmd/promptline/main.go at 12KB. Would you like me to read it?
```

## Troubleshooting

### Tool not being called
- Check that the AI knows about it (it's in the system prompt)
- Verify the tool name matches exactly
- Ensure the description is clear

### Parameter parsing errors
- Remember JSON numbers are `float64`, not `int`
- Use type assertions with proper error handling
- Provide defaults for optional parameters

### Tool execution fails
- Add detailed error messages
- Log errors for debugging
- Test your tool function independently

## File Organization

```
internal/tools/
├── tools.go         # Core framework (don't modify)
│   ├── ExecutorFunc type
│   ├── Tool struct
│   ├── Registry
│   └── Parsing & execution logic
│
└── builtin.go       # Tool implementations (add here)
    ├── registerBuiltInTools()  # Register new tools
    ├── getCurrentDatetime()
    ├── executeShellCommand()
    ├── readFile()
    ├── writeFile()
    └── [YOUR NEW TOOL]()
```

## Next Steps

1. Think about what tools would be useful for your use case
2. Implement them following this guide
3. Test thoroughly
4. Consider security implications
5. Document your tools in `FUNCTION_CALLING.md`

Happy tool building! 🛠️
