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
	"strings"
	"testing"
)

type validationFixture struct {
	Name  string `json:"name" validate:"required,min=2,max=5"`
	Mode  string `json:"mode" validate:"oneof=alpha beta"`
	Count int    `json:"count" validate:"min=1"`
}

func TestUnmarshalAndValidateCreateFileArgs(t *testing.T) {
	_, err := unmarshalAndValidate[createFileArgs](map[string]interface{}{
		"path":    "example.txt",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestUnmarshalAndValidateCreateFileArgsMissingPath(t *testing.T) {
	_, err := unmarshalAndValidate[createFileArgs](map[string]interface{}{
		"content": "hello",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'path'") {
		t.Fatalf("expected path error, got %v", err)
	}
}

func TestUnmarshalAndValidateCreateFileArgsTypeMismatch(t *testing.T) {
	_, err := unmarshalAndValidate[createFileArgs](map[string]interface{}{
		"path":    123,
		"content": "hello",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'path'") {
		t.Fatalf("expected path error, got %v", err)
	}
}

func TestUnmarshalAndValidateFixtureRequired(t *testing.T) {
	_, err := unmarshalAndValidate[validationFixture](map[string]interface{}{
		"mode":  "alpha",
		"count": 1,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'name'") {
		t.Fatalf("expected name error, got %v", err)
	}
}

func TestUnmarshalAndValidateFixtureMinMax(t *testing.T) {
	_, err := unmarshalAndValidate[validationFixture](map[string]interface{}{
		"name":  "a",
		"mode":  "alpha",
		"count": 1,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'name'") {
		t.Fatalf("expected name error, got %v", err)
	}

	_, err = unmarshalAndValidate[validationFixture](map[string]interface{}{
		"name":  "toolong",
		"mode":  "alpha",
		"count": 1,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'name'") {
		t.Fatalf("expected name error, got %v", err)
	}
}

func TestUnmarshalAndValidateFixtureOneOf(t *testing.T) {
	_, err := unmarshalAndValidate[validationFixture](map[string]interface{}{
		"name":  "okay",
		"mode":  "gamma",
		"count": 1,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'mode'") {
		t.Fatalf("expected mode error, got %v", err)
	}
}

func TestUnmarshalAndValidateFixtureMinValue(t *testing.T) {
	_, err := unmarshalAndValidate[validationFixture](map[string]interface{}{
		"name":  "okay",
		"mode":  "alpha",
		"count": 0,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "'count'") {
		t.Fatalf("expected count error, got %v", err)
	}
}
