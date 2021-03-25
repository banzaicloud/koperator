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

	. "github.com/onsi/gomega"
)

const (
	testPropWithEscapedSep     = "\\=test\\:key\\=test\\ value\\:"
	testPropWithRemovedEscapes = "=test:key=test value:"
)

func TestGetSeparator(t *testing.T) {

	t.Run("Found '=' separator", func(t *testing.T) {
		g := NewGomegaWithT(t)

		s := "="
		i := 8
		prop := "test.key=test.value"

		sep, idx, err := GetSeparator(prop)
		g.Expect(err).Should(Succeed())
		g.Expect(sep).Should(Equal(s))
		g.Expect(idx).Should(Equal(i))
	})

	t.Run("Found ':' separator", func(t *testing.T) {
		g := NewGomegaWithT(t)

		s := ":"
		i := 8
		prop := "test.key:test.value"

		sep, idx, err := GetSeparator(prop)
		g.Expect(err).Should(Succeed())
		g.Expect(sep).Should(Equal(s))
		g.Expect(idx).Should(Equal(i))
	})

	t.Run("Found ' ' separator", func(t *testing.T) {
		g := NewGomegaWithT(t)

		s := " "
		i := 8
		prop := "test.key test.value"

		sep, idx, err := GetSeparator(prop)
		g.Expect(err).Should(Succeed())
		g.Expect(sep).Should(Equal(s))
		g.Expect(idx).Should(Equal(i))
	})

	t.Run("No separator", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := "test.key,test.value"

		_, _, err := GetSeparator(prop)
		g.Expect(err).Should(HaveOccurred())
	})

	t.Run("No string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := ""

		_, _, err := GetSeparator(prop)
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestUnEscapeSeparators(t *testing.T) {

	t.Run("Remove escaping of separators", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := testPropWithEscapedSep
		expected := testPropWithRemovedEscapes

		result := UnEscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})

	t.Run("Do nothing", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := testPropWithRemovedEscapes
		expected := testPropWithRemovedEscapes

		result := UnEscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})

	t.Run("Empty string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := ""
		expected := ""

		result := UnEscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})
}

func TestEscapeSeparators(t *testing.T) {

	t.Run("Escaping separators", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := testPropWithRemovedEscapes
		expected := testPropWithEscapedSep

		result := EscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})

	t.Run("Do nothing", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := testPropWithEscapedSep
		expected := testPropWithEscapedSep

		result := EscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})

	t.Run("Empty string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := ""
		expected := ""

		result := EscapeSeparators(prop)
		g.Expect(result).Should(Equal(expected))
	})
}

func TestNewFromString(t *testing.T) {

	propString := `# Comment
test.key=test.value
! Comment
test.key2:test.value2
test.key3 test.value3
test.key4=test.value41 \
test=value42 \
test:value43 \
test value44

`
	p, err := NewFromString(propString)

	t.Run("Getting Properties from string result no error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		g.Expect(err).Should(Succeed())
	})

	t.Run("Get Properties from string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := []string{
			"test.key",
			"test.key2",
			"test.key3",
			"test.key4",
		}

		g.Expect(p.Keys()).Should(Equal(expected))
	})

	t.Run("Multiline Property", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := "test.key4"
		expected := "test.value41 test=value42 test:value43 test value44"

		v, _ := p.Get(prop)

		g.Expect(v.Value()).Should(Equal(expected))
	})

	t.Run("Invalid property string should trigger an error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		invalidProp := "INVALID.PROPERTY"

		_, err := NewFromString(invalidProp)
		g.Expect(err).Should(HaveOccurred())
	})

	t.Run("Empty string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &Properties{
			properties: map[string]Property{},
			keys:       map[string]keyIndex{},
		}

		p, err := NewFromString("")
		g.Expect(err).Should(Succeed())
		g.Expect(p).Should(Equal(expected))
		g.Expect(p.Len()).Should(Equal(0))
	})
}

func TestNewPropertyFromString(t *testing.T) {

	t.Run("Parse valid property string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := "test.key=test value"
		expected := Property{
			key:     "test.key",
			value:   "test value",
			comment: "",
		}

		p, err := newPropertyFromString(prop, "")
		g.Expect(err).Should(Succeed())
		g.Expect(p).Should(Equal(expected))
	})

	t.Run("Parse valid property string with escaped key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := "test\\:key=test value"
		expected := Property{
			key:   "test:key",
			value: "test value",
		}

		p, err := newPropertyFromString(prop, "")
		g.Expect(err).Should(Succeed())
		g.Expect(p).Should(Equal(expected))
	})

	t.Run("Parse invalid property string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := "test.key.test.value"

		_, err := newPropertyFromString(prop, "")
		g.Expect(err).Should(HaveOccurred())
	})
}
