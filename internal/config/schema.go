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

package config

import (
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaJSON returns the JSON schema for config.json.
func SchemaJSON() string {
	return configSchemaJSON
}

// ExampleConfigJSON returns a minimal example config derived from the schema.
func ExampleConfigJSON() string {
	return exampleConfigJSON
}

func normalizeConfigJSON(data []byte) ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	migrateLegacyConfig(raw)
	if err := validateConfigMap(raw, ""); err != nil {
		return nil, err
	}
	normalized, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func migrateLegacyConfig(raw map[string]interface{}) {
	toolsVal, ok := raw["tools"].(map[string]interface{})
	if !ok {
		return
	}
	if _, ok := toolsVal["require_confirmation"]; ok {
		return
	}
	if legacy, ok := toolsVal["confirm"].([]interface{}); ok {
		toolsVal["require_confirmation"] = legacy
		delete(toolsVal, "confirm")
	}
}

func validateConfigMap(raw map[string]interface{}, prefix string) error {
	allowed := map[string]func(interface{}) error{
		"api_key": func(v interface{}) error { return validateString(v, prefix+"api_key") },
		"api_url": func(v interface{}) error { return validateString(v, prefix+"api_url") },
		"model":   func(v interface{}) error { return validateString(v, prefix+"model") },
		"temperature": func(v interface{}) error {
			return validateNumber(v, prefix+"temperature")
		},
		"max_tokens": func(v interface{}) error { return validateNumber(v, prefix+"max_tokens") },
		"history_file": func(v interface{}) error {
			return validateString(v, prefix+"history_file")
		},
		"command_history_file": func(v interface{}) error {
			return validateString(v, prefix+"command_history_file")
		},
		"history_max_messages": func(v interface{}) error {
			return validateNumber(v, prefix+"history_max_messages")
		},
		"tools": func(v interface{}) error {
			return validateToolsConfig(v, prefix+"tools.")
		},
		"tool_limits": func(v interface{}) error {
			return validateToolLimits(v, prefix+"tool_limits.")
		},
		"tool_path_whitelist": func(v interface{}) error {
			return validateStringArray(v, prefix+"tool_path_whitelist")
		},
		"tool_rate_limits": func(v interface{}) error {
			return validateToolRateLimits(v, prefix+"tool_rate_limits.")
		},
		"tool_timeouts": func(v interface{}) error {
			return validateToolTimeouts(v, prefix+"tool_timeouts.")
		},
		"tool_output_filters": func(v interface{}) error {
			return validateToolOutputFilters(v, prefix+"tool_output_filters.")
		},
	}

	for key, value := range raw {
		validator, ok := allowed[key]
		if !ok {
			return fmt.Errorf("unknown configuration field %q", key)
		}
		if err := validator(value); err != nil {
			return err
		}
	}

	return nil
}

func validateToolsConfig(value interface{}, prefix string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%stools must be an object", prefix)
	}
	allowed := map[string]func(interface{}) error{
		"allow":                func(v interface{}) error { return validateStringArray(v, prefix+"allow") },
		"ask":                  func(v interface{}) error { return validateStringArray(v, prefix+"ask") },
		"deny":                 func(v interface{}) error { return validateStringArray(v, prefix+"deny") },
		"require_confirmation": func(v interface{}) error { return validateStringArray(v, prefix+"require_confirmation") },
	}
	for key, val := range section {
		validator, ok := allowed[key]
		if !ok {
			return fmt.Errorf("unknown configuration field %q", prefix+key)
		}
		if err := validator(val); err != nil {
			return err
		}
	}
	return nil
}

func validateToolLimits(value interface{}, prefix string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%stool_limits must be an object", prefix)
	}
	allowed := map[string]func(interface{}) error{
		"max_file_size_bytes":   func(v interface{}) error { return validateNumber(v, prefix+"max_file_size_bytes") },
		"max_directory_depth":   func(v interface{}) error { return validateNumber(v, prefix+"max_directory_depth") },
		"max_directory_entries": func(v interface{}) error { return validateNumber(v, prefix+"max_directory_entries") },
	}
	return validateSection(section, allowed, prefix)
}

func validateToolRateLimits(value interface{}, prefix string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%stool_rate_limits must be an object", prefix)
	}
	allowed := map[string]func(interface{}) error{
		"default_per_minute": func(v interface{}) error { return validateNumber(v, prefix+"default_per_minute") },
		"per_tool":           func(v interface{}) error { return validateStringNumberMap(v, prefix+"per_tool") },
		"cooldown_seconds":   func(v interface{}) error { return validateStringNumberMap(v, prefix+"cooldown_seconds") },
	}
	return validateSection(section, allowed, prefix)
}

func validateToolTimeouts(value interface{}, prefix string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%stool_timeouts must be an object", prefix)
	}
	allowed := map[string]func(interface{}) error{
		"default_seconds":  func(v interface{}) error { return validateNumber(v, prefix+"default_seconds") },
		"per_tool_seconds": func(v interface{}) error { return validateStringNumberMap(v, prefix+"per_tool_seconds") },
	}
	return validateSection(section, allowed, prefix)
}

func validateToolOutputFilters(value interface{}, prefix string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%stool_output_filters must be an object", prefix)
	}
	allowed := map[string]func(interface{}) error{
		"max_chars":     func(v interface{}) error { return validateNumber(v, prefix+"max_chars") },
		"strip_ansi":    func(v interface{}) error { return validateBool(v, prefix+"strip_ansi") },
		"strip_control": func(v interface{}) error { return validateBool(v, prefix+"strip_control") },
	}
	return validateSection(section, allowed, prefix)
}

func validateSection(section map[string]interface{}, allowed map[string]func(interface{}) error, prefix string) error {
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		validator, ok := allowed[key]
		if !ok {
			return fmt.Errorf("unknown configuration field %q", prefix+key)
		}
		if err := validator(section[key]); err != nil {
			return err
		}
	}
	return nil
}

func validateString(value interface{}, name string) error {
	if _, ok := value.(string); !ok {
		return fmt.Errorf("%s must be a string", name)
	}
	return nil
}

func validateNumber(value interface{}, name string) error {
	if _, ok := value.(float64); !ok {
		return fmt.Errorf("%s must be a number", name)
	}
	return nil
}

func validateBool(value interface{}, name string) error {
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("%s must be a boolean", name)
	}
	return nil
}

func validateStringArray(value interface{}, name string) error {
	list, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("%s must be an array of strings", name)
	}
	for _, item := range list {
		if _, ok := item.(string); !ok {
			return fmt.Errorf("%s must be an array of strings", name)
		}
	}
	return nil
}

func validateStringNumberMap(value interface{}, name string) error {
	section, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s must be an object of number values", name)
	}
	for key, entry := range section {
		if _, ok := entry.(float64); !ok {
			return fmt.Errorf("%s.%s must be a number", name, key)
		}
	}
	return nil
}

const configSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Promptline Config",
  "type": "object",
  "properties": {
    "api_key": { "type": "string" },
    "api_url": { "type": "string" },
    "model": { "type": "string" },
    "temperature": { "type": "number" },
    "max_tokens": { "type": "number" },
    "history_file": { "type": "string" },
    "command_history_file": { "type": "string" },
    "history_max_messages": { "type": "number" },
    "tools": {
      "type": "object",
      "properties": {
        "allow": { "type": "array", "items": { "type": "string" } },
        "ask": { "type": "array", "items": { "type": "string" } },
        "deny": { "type": "array", "items": { "type": "string" } },
        "require_confirmation": { "type": "array", "items": { "type": "string" } }
      }
    },
    "tool_limits": {
      "type": "object",
      "properties": {
        "max_file_size_bytes": { "type": "number" },
        "max_directory_depth": { "type": "number" },
        "max_directory_entries": { "type": "number" }
      }
    },
    "tool_path_whitelist": { "type": "array", "items": { "type": "string" } },
    "tool_rate_limits": {
      "type": "object",
      "properties": {
        "default_per_minute": { "type": "number" },
        "per_tool": { "type": "object", "additionalProperties": { "type": "number" } },
        "cooldown_seconds": { "type": "object", "additionalProperties": { "type": "number" } }
      }
    },
    "tool_timeouts": {
      "type": "object",
      "properties": {
        "default_seconds": { "type": "number" },
        "per_tool_seconds": { "type": "object", "additionalProperties": { "type": "number" } }
      }
    },
    "tool_output_filters": {
      "type": "object",
      "properties": {
        "max_chars": { "type": "number" },
        "strip_ansi": { "type": "boolean" },
        "strip_control": { "type": "boolean" }
      }
    }
  }
}`

const exampleConfigJSON = `{
  "api_key": "sk-...",
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "tools": {
    "allow": ["get_current_datetime", "read_file", "ls"],
    "ask": [
      "write_file",
      "cat",
      "cp",
      "mv",
      "rm",
      "ln",
      "touch",
      "truncate",
      "readlink",
      "realpath",
      "mkdir",
      "pwd",
      "dirname",
      "basename",
      "grep",
      "head",
      "tail",
      "sort",
      "uniq",
      "wc",
      "tr",
      "tee",
      "comm",
      "strings",
      "more",
      "hexdump",
      "cmp",
      "md5sum",
      "shasum",
      "base64",
      "uname",
      "hostname",
      "uptime",
      "free",
      "df",
      "du",
      "ps",
      "pidof",
      "id",
      "echo",
      "seq",
      "printenv",
      "tty",
      "which",
      "mkfifo",
      "mktemp",
      "find",
      "chmod",
      "date"
    ]
  }
}`
