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

type TestUnmarshalStruct struct {
	StringField string   `properties:"string.field"`
	IntField    int64    `properties:"int.field"`
	BoolField   bool     `properties:"bool.field"`
	FloatField  float64  `properties:"float.field"`
	ListField   []string `properties:"list.field"`

	CustomField string `properties:"custom.field"`

	NonPropertyField string

	EmptyPropertyField string `properties:""`
	SkipPropertyField  string `properties:"-"`
}

type TestUnmarshalerStruct struct {
	CustomField string
}

func (t *TestUnmarshalerStruct) UnmarshalProperties(p *Properties) error {
	if um, found := p.Get("custom.field"); found {
		t.CustomField = um.Value()
	}
	return nil
}

func TestUnmarshal(t *testing.T) {
	t.Run("Nil interface", func(t *testing.T) {

		p := NewProperties()

		if err := Unmarshal(p, nil); err == nil {
			t.Errorf("Unmarshal should return with error!\n %v", err)
		}
	})

	t.Run("Nil pointer", func(t *testing.T) {
		var s *TestUnmarshalStruct

		p := NewProperties()

		if err := Unmarshal(p, s); err == nil {
			t.Errorf("Unmarshal should return with error!\n %v", err)
		}
	})

	t.Run("Load to non struct type", func(t *testing.T) {
		var s string

		p := NewProperties()

		if err := Unmarshal(p, s); err == nil {
			t.Errorf("Unmarshal should return with error!\n %v", err)
		}

		if err := Unmarshal(p, &s); err == nil {
			t.Errorf("Unmarshal should return with error!\n %v", err)
		}
	})

	t.Run("Load to struct", func(t *testing.T) {
		s := &TestUnmarshalStruct{}

		expectedString := "property string"
		expectedInt := int64(100)
		expectedBool := true
		expectedFloat := 128.9
		expectedList := []string{"test value1", "test value2"}

		p := NewProperties()
		_ = p.Set("string.field", expectedString)
		_ = p.Set("int.field", expectedInt)
		_ = p.Set("bool.field", expectedBool)
		_ = p.Set("float.field", expectedFloat)
		_ = p.Set("list.field", expectedList)

		err := Unmarshal(p, s)

		if err != nil {
			t.Errorf("Unmarshal should not return with error!\n %v", err)
		}

		if s.StringField != expectedString {
			t.Errorf("Mismatch in expected and actual value of struct field!\nExpected: %v\nGot: %v\n", expectedString, s.StringField)
		}

		if s.IntField != expectedInt {
			t.Errorf("Mismatch in expected and actual value of struct field!\nExpected: %v\nGot: %v\n", expectedInt, s.IntField)
		}

		if s.BoolField != expectedBool {
			t.Errorf("Mismatch in expected and actual value of struct field!\nExpected: %v\nGot: %v\n", expectedBool, s.BoolField)
		}

		if s.FloatField != expectedFloat {
			t.Errorf("Mismatch in expected and actual value of struct field!\nExpected: %v\nGot: %v\n", expectedFloat, s.FloatField)
		}

		if !cmp.Equal(s.ListField, expectedList) {
			t.Errorf("Mismatch in expected and actual value of struct field!\nExpected: %v\nGot: %v\n", expectedList, s.ListField)
		}
	})

	t.Run("Struct implementing Unmarshaler interface", func(t *testing.T) {
		s := &TestUnmarshalerStruct{
			CustomField: "before unmarshal",
		}

		expectedValue := "after unmarshal"
		p := NewProperties()
		_ = p.Set("custom.field", expectedValue)

		err := Unmarshal(p, s)

		if err != nil {
			t.Errorf("Marshal should not return error!")
		}

		if s.CustomField != expectedValue {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n", expectedValue, s.CustomField)
		}
	})
}
