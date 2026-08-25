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

import "context"

// Tool represents a callable tool/function with validation and execution hooks.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
	Validate(args map[string]interface{}) error
}

// ToolDefinition provides a default implementation of Tool.
type ToolDefinition struct {
	NameValue        string
	DescriptionValue string
	ParametersValue  map[string]interface{}
	ExecuteFunc      ExecutorFunc
	ValidateFunc     func(args map[string]interface{}) error
}

var _ Tool = (*ToolDefinition)(nil)

func (t *ToolDefinition) Name() string {
	return t.NameValue
}

func (t *ToolDefinition) Description() string {
	return t.DescriptionValue
}

func (t *ToolDefinition) Parameters() map[string]interface{} {
	return t.ParametersValue
}

func (t *ToolDefinition) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ExecuteFunc == nil {
		return "", nil
	}
	return t.ExecuteFunc(ctx, args)
}

func (t *ToolDefinition) Validate(args map[string]interface{}) error {
	if t.ValidateFunc == nil {
		return nil
	}
	return t.ValidateFunc(args)
}
