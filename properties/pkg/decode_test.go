// Copyright © 2021 Cisco Systems, Inc. and/or its affiliates
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

	. "github.com/onsi/gomega"
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
		g := NewGomegaWithT(t)
		p := NewProperties()
		err := Unmarshal(p, nil)

		g.Expect(err).Should(HaveOccurred(),
			"Unmarshal should return an error for nil interface!")
	})

	t.Run("Empty struct pointer", func(t *testing.T) {
		var s *TestUnmarshalStruct

		g := NewGomegaWithT(t)
		p := NewProperties()
		err := Unmarshal(p, s)

		g.Expect(err).Should(HaveOccurred(),
			"Passing empty struct pointer to unmarshal should return an error!")
	})

	t.Run("Load to non struct type", func(t *testing.T) {
		var s string

		g := NewGomegaWithT(t)
		p := NewProperties()
		err := Unmarshal(p, s)

		g.Expect(err).Should(HaveOccurred(),
			"Passing string to unmarshal should return an error!")

		err = Unmarshal(p, &s)

		g.Expect(err).Should(HaveOccurred(),
			"Passing string pointer to unmarshal should return an error!")
	})

	t.Run("Unmarshal to struct", func(t *testing.T) {
		g := NewGomegaWithT(t)

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

		g.Expect(err).Should(Succeed())

		g.Expect(s.StringField).Should(Equal(expectedString))

		g.Expect(s.IntField).Should(Equal(expectedInt))

		g.Expect(s.BoolField).Should(Equal(expectedBool))

		g.Expect(s.FloatField).Should(Equal(expectedFloat))

		g.Expect(s.ListField).Should(Equal(expectedList))
	})

	t.Run("Struct implementing Unmarshaler interface", func(t *testing.T) {
		g := NewGomegaWithT(t)

		s := &TestUnmarshalerStruct{
			CustomField: "before unmarshal",
		}

		expectedValue := "after unmarshal"
		p := NewProperties()
		_ = p.Set("custom.field", expectedValue)

		err := Unmarshal(p, s)

		g.Expect(err).Should(Succeed())

		g.Expect(s.CustomField).Should(Equal(expectedValue))
	})
}
