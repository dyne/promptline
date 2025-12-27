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
	"reflect"

	"github.com/invopop/jsonschema"
)

func mustSchemaParametersFor[T any]() map[string]interface{} {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		panic("schema type is nil")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	params, err := schemaParametersForType(t)
	if err != nil {
		panic(err)
	}
	return params
}

func schemaParametersForType(t reflect.Type) (map[string]interface{}, error) {
	defName := t.Name()
	schema := jsonschema.ReflectFromType(t)
	def, ok := schema.Definitions[defName]
	if !ok {
		return nil, fmt.Errorf("schema definition %q not found", defName)
	}
	params := &jsonschema.Schema{
		Type:       "object",
		Properties: def.Properties,
		Required:   def.Required,
	}
	return jsonSchemaToMap(params)
}

func jsonSchemaToMap(schema interface{}) (map[string]interface{}, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var params map[string]interface{}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	return params, nil
}
