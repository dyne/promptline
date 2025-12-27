## Validation

Promptline applies validation in layers to guard against malformed tool arguments,
especially when using untrusted LLM providers.

### Defense-in-depth flow

1. Provider schema enforcement (best-effort)
2. Local struct validation via `go-playground/validator/v10`
3. Custom validators and security checks in tool implementations

The local validator layer protects against buggy providers or local models that
ignore JSON Schema constraints. Custom validators still run to enforce security
rules like path whitelisting and file size limits.

### Adding validation to a tool

1. Define a struct for the tool arguments with `json` and `jsonschema` tags.
2. Add `validate` tags for required fields and bounds.
3. Use `unmarshalAndValidate[T]` before custom validation in the tool handler.

Common tag patterns:

- `required` for mandatory fields
- `min` / `max` for string length or numeric bounds
- `oneof=val1 val2` for enums
- `omitempty` to skip validation when a field is absent

### When validator matters most

Validator checks are most valuable when:

- Using local models (Ollama, LM Studio)
- Integrating alternative providers (Claude, custom APIs)
- Handling tool calls from untrusted sources
