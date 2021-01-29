// Copyright © 2021 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package properties

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type TestStruct struct {
	StringField string   `properties:"string.field"`
	IntField    int64    `properties:"int.field"`
	BoolField   bool     `properties:"bool.field"`
	FloatField  float64  `properties:"float.field"`
	ListField   []string `properties:"list.field"`

	NonPropertyField string

	EmptyPropertyField string `properties:""`
	OmitPropertyField  string `properties:"omitempty.field,omitempty"`
	SkipPropertyField  string `properties:"-"`
}

type TestStructWithInvalidListField struct {
	StringField string `properties:"string.field"`

	InvalidListField []int `properties:"list.field"`
}

type TestMarshalerStruct struct {
	StringField string   `properties:"string.field"`
	IntField    int64    `properties:"int.field"`
	BoolField   bool     `properties:"bool.field"`
	FloatField  float64  `properties:"float.field"`
	ListField   []string `properties:"list.field"`
}

func (s TestMarshalerStruct) MarshalProperties() (*Properties, error) {
	p := NewProperties()
	_ = p.Set("custom.marshaller.called", "true")
	return p, nil
}

func TestMarshal(t *testing.T) {
	t.Run("Nil value", func(t *testing.T) {
		if _, err := Marshal(nil); err == nil {
			t.Error("Marshal should return with error!")
		}
	})

	t.Run("Nil-pointer", func(t *testing.T) {
		var s *TestStruct = nil

		if _, err := Marshal(s); err == nil {
			t.Error("Marshal should return with error!")
		}
	})

	t.Run("Non-struct value", func(t *testing.T) {
		s := []string{"item1", "item2"}

		if _, err := Marshal(s); err == nil {
			t.Error("Marshal should return with error!")
		}
	})

	t.Run("Empty struct", func(t *testing.T) {
		s := TestStruct{}
		if _, err := Marshal(s); err != nil {
			t.Errorf("Marshal should not return error!\n %v", err)
		}
	})

	t.Run("Pointer to empty struct", func(t *testing.T) {
		s := TestStruct{}
		if _, err := Marshal(&s); err != nil {
			t.Errorf("Marshal should not return error!\n %v", err)
		}
	})

	t.Run("Struct", func(t *testing.T) {
		s := TestStruct{
			StringField:        "property string",
			IntField:           100,
			BoolField:          true,
			FloatField:         128.9,
			ListField:          []string{"test value1", "test value2"},
			NonPropertyField:   "non property field",
			EmptyPropertyField: "empty property field",
			SkipPropertyField:  "skip property field",
		}
		p, err := Marshal(s)

		if err != nil {
			t.Errorf("Marshal should not return error!\n %v", err)
		}

		expected := NewProperties()
		_ = expected.Set("string.field", "property string")
		_ = expected.Set("int.field", 100)
		_ = expected.Set("bool.field", true)
		_ = expected.Set("float.field", 128.9)
		_ = expected.Set("list.field", []string{"test value1", "test value2"})

		if !p.Equal(expected) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n", expected, p)
		}
	})

	t.Run("Pointer to struct", func(t *testing.T) {
		s := &TestStruct{
			StringField:        "property string",
			IntField:           100,
			BoolField:          true,
			FloatField:         128.9,
			ListField:          []string{"test value1", "test value2"},
			NonPropertyField:   "non property field",
			EmptyPropertyField: "empty property field",
			OmitPropertyField:  "omitempty property field",
			SkipPropertyField:  "skip property field",
		}
		p, err := Marshal(s)

		if err != nil {
			t.Errorf("Marshal should not return error!\n %v", err)
		}

		expected := NewProperties()
		_ = expected.Set("string.field", "property string")
		_ = expected.Set("int.field", 100)
		_ = expected.Set("bool.field", true)
		_ = expected.Set("float.field", 128.9)
		_ = expected.Set("list.field", "test value1,test value2")
		_ = expected.Set("omitempty.field", "omitempty property field")

		if !cmp.Equal(p, expected, cmp.AllowUnexported(Properties{}), cmp.AllowUnexported(Property{})) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n\n %v\n", expected, p, cmp.Diff(expected, p))
		}
	})

	t.Run("Struct with invalid list field", func(t *testing.T) {
		s := TestStructWithInvalidListField{
			StringField:      "property string",
			InvalidListField: []int{1, 2, 3},
		}

		if _, err := Marshal(s); err == nil {
			t.Error("Marshal should return with error!")
		}
	})

	t.Run("Struct implementing Marshaler interface", func(t *testing.T) {
		s := &TestMarshalerStruct{
			StringField: "property string",
			IntField:    100,
			BoolField:   true,
			FloatField:  128.9,
			ListField:   []string{"test value1", "test value2"},
		}
		p, err := Marshal(s)

		if err != nil {
			t.Error("Marshal should return with error!")
		}

		expected := NewProperties()
		_ = expected.Set("custom.marshaller.called", "true")

		if !cmp.Equal(p, expected, cmp.AllowUnexported(Properties{}), cmp.AllowUnexported(Property{})) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n\n %v\n", expected, p, cmp.Diff(expected, p))
		}
	})
}
