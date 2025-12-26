// Copyright (C) 2025 Dyne.org foundation
// designed, written and maintained by Denis Roio <jaromil@dyne.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationRule checks tool arguments and returns an error if invalid.
type ValidationRule func(args map[string]interface{}) error

// ValidateToolCall validates a tool call before execution.
func (r *Registry) ValidateToolCall(name, argsJSON string) *ToolResult {
	tool, ok := r.getTool(name)
	if !ok {
		return invalidToolResult(name, fmt.Errorf("%w: tool %q not found", ErrToolNotFound, name))
	}

	args, err := parseToolArgs(argsJSON)
	if err != nil {
		return invalidToolResult(name, fmt.Errorf("%w: %v", ErrInvalidArguments, err))
	}

	if err := tool.Validate(args); err != nil {
		return invalidToolResult(name, fmt.Errorf("%w: %v", ErrInvalidArguments, err))
	}

	return nil
}

func invalidToolResult(name string, err error) *ToolResult {
	return &ToolResult{
		Function: name,
		Result:   fmt.Sprintf("Error: %v", err),
		Error:    err,
	}
}

func parseToolArgs(argsJSON string) (map[string]interface{}, error) {
	args := map[string]interface{}{}
	if strings.TrimSpace(argsJSON) == "" {
		return args, nil
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, err
	}
	return args, nil
}

// ChainValidation runs rules in order until the first error.
func ChainValidation(rules ...ValidationRule) ValidationRule {
	return func(args map[string]interface{}) error {
		for _, rule := range rules {
			if rule == nil {
				continue
			}
			if err := rule(args); err != nil {
				return err
			}
		}
		return nil
	}
}

// RequireStringArg ensures a string argument is present and non-empty.
func RequireStringArg(key, message string) ValidationRule {
	return func(args map[string]interface{}) error {
		value, ok := args[key]
		if !ok || value == nil {
			return fmt.Errorf("%s", message)
		}
		str, ok := value.(string)
		if !ok || strings.TrimSpace(str) == "" {
			return fmt.Errorf("%s", message)
		}
		return nil
	}
}

// RequireNonEmptyArg ensures an argument is present and non-empty.
func RequireNonEmptyArg(key, message string) ValidationRule {
	return func(args map[string]interface{}) error {
		value, ok := args[key]
		if !ok || value == nil {
			return fmt.Errorf("%s", message)
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%s", message)
			}
		case []interface{}:
			if len(v) == 0 {
				return fmt.Errorf("%s", message)
			}
		case map[string]interface{}:
			if len(v) == 0 {
				return fmt.Errorf("%s", message)
			}
		}
		return nil
	}
}
