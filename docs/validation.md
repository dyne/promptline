## Validation

Promptline validates arguments received through its bounded Codex app-server
and toolbox MCP boundaries.

### Defense-in-depth flow

1. Stable MCP/app-server schema validation
2. Local struct validation via `go-playground/validator/v10`
3. Custom validators, scoped-root checks, limits, and approval policy

The local validator layer protects against malformed or unexpected protocol
input. Custom validators still enforce scoped-root authority and bounded file
and traversal limits.

### Adding validation to a tool

1. Define a struct for the tool arguments with `json` and `jsonschema` tags.
2. Add `validate` tags for required fields and bounds.
3. Use `unmarshalAndValidate[T]` before custom validation in the tool handler.

Common tag patterns:

- `required` for mandatory fields
- `min` / `max` for string length or numeric bounds
- `oneof=val1 val2` for enums
- `omitempty` to skip validation when a field is absent

### When validation matters most

Validator checks are most valuable when:

- Handling malformed MCP requests
- Enforcing filesystem and resource boundaries
- Maintaining compatibility with a versioned Codex app-server protocol
