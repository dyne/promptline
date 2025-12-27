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

	"github.com/567-labs/instructor-go/pkg/instructor"
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
	schema, err := instructor.NewSchema(t)
	if err != nil {
		return nil, err
	}

	defName := t.Name()
	for _, fn := range schema.Functions {
		if fn.Name != defName {
			continue
		}
		return jsonSchemaToMap(fn.Parameters)
	}

	return nil, fmt.Errorf("schema definition %q not found", defName)
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
