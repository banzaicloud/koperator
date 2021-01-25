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
	"reflect"
	"testing"
)

func TestGetSeparator(t *testing.T) {

	t.Run("Found '=' separator", func(t *testing.T) {
		s := "="
		i := 8
		prop := "test.key=test.value"

		sep, idx, err := GetSeparator(prop)

		if err != nil {
			t.Errorf("Finding %q separator in %q string resulted an error: %v", s, prop, err)
		}

		if sep != s {
			t.Errorf("Returned separator does not match. Expected: %v, got %v", s, sep)
		}

		if idx != i {
			t.Errorf("Returned index of the spearator does not match. Expected: %v, got %v", i, idx)
		}
	})

	t.Run("Found ':' separator", func(t *testing.T) {
		s := ":"
		i := 8
		prop := "test.key:test.value"

		sep, idx, err := GetSeparator(prop)

		if err != nil {
			t.Errorf("Finding %q separator in %q string resulted an error: %v", s, prop, err)
		}

		if sep != s {
			t.Errorf("Returned separator does not match. Expected: %v, got %v", s, sep)
		}

		if idx != i {
			t.Errorf("Returned index of the spearator does not match. Expected: %v, got %v", i, idx)
		}
	})

	t.Run("Found ' ' separator", func(t *testing.T) {
		s := " "
		i := 8
		prop := "test.key test.value"

		sep, idx, err := GetSeparator(prop)

		if err != nil {
			t.Errorf("Finding %q separator in %q string resulted an error: %v", s, prop, err)
		}

		if sep != s {
			t.Errorf("Returned separator does not match. Expected: %v, got %v", s, sep)
		}

		if idx != i {
			t.Errorf("Returned index of the spearator does not match. Expected: %v, got %v", i, idx)
		}
	})

	t.Run("No separator", func(t *testing.T) {
		prop := "test.key,test.value"

		_, _, err := GetSeparator(prop)

		if err == nil {
			t.Errorf("Finding separator in invalid Property string should trigger an error!")
		}
	})

	t.Run("No string", func(t *testing.T) {
		prop := ""
		_, _, err := GetSeparator(prop)

		if err == nil {
			t.Errorf("Finding separator in invalid Property string should trigger an error!")
		}
	})
}

func TestUnEscapeSeparators(t *testing.T) {

	t.Run("Remove escaping of separators", func(t *testing.T) {
		prop := "\\=test\\:key\\=test\\ value\\:"
		expected := "=test:key=test value:"

		result := UnEscapeSeparators(prop)

		if expected != result {
			t.Errorf("Removing escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
	})

	t.Run("Do nothing", func(t *testing.T) {
		prop := "=test:key=test value:"
		expected := "=test:key=test value:"

		result := UnEscapeSeparators(prop)

		if expected != result {
			t.Errorf("Removing escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		prop := ""
		expected := ""

		result := UnEscapeSeparators(prop)

		if expected != result {
			t.Errorf("Removing escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
	})
}

func TestEscapeSeparators(t *testing.T) {

	t.Run("Escaping separators", func(t *testing.T) {
		prop := "=test:key=test value:"
		expected := "\\=test\\:key\\=test\\ value\\:"

		result := EscapeSeparators(prop)

		if expected != result {
			t.Errorf("Escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
	})

	t.Run("Do nothing", func(t *testing.T) {
		prop := "\\=test\\:key\\=test\\ value\\:"
		expected := "\\=test\\:key\\=test\\ value\\:"

		result := EscapeSeparators(prop)

		if expected != result {
			t.Errorf("Escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		prop := ""
		expected := ""

		result := EscapeSeparators(prop)

		if expected != result {
			t.Errorf("Removing escaping of separators resulted a mismatch. Expected %v, got %v", expected, result)
		}
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
		if err != nil {
			t.Errorf("Parsing valid Properties string should not result an error: %v", err)
		}
	})

	t.Run("Get Properties from string", func(t *testing.T) {

		expected := []string{
			"test.key",
			"test.key2",
			"test.key3",
			"test.key4",
		}

		k := p.Keys()

		if !reflect.DeepEqual(k, expected) {
			t.Errorf("Keys in Properties mismatch. Expected %v, got %v", expected, k)
		}
	})

	t.Run("Multiline Property", func(t *testing.T) {
		prop := "test.key4"
		expected := "test.value41 test=value42 test:value43 test value44"

		v, _ := p.Get(prop)

		if !reflect.DeepEqual(v.Value(), expected) {
			t.Errorf("Value of multiline Property does not match. Expected %v, got %v", expected, v.Value())
		}
	})

	t.Run("Invalid property string should trigger an error", func(t *testing.T) {
		invalidProp := "INVALID.PROPERTY"
		_, err := NewFromString(invalidProp)

		if err == nil {
			t.Errorf("Parsing invalid Properties should trigger an InvalidPropertyError, but it did not.")
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		invalidProp := ""
		_, err := NewFromString(invalidProp)

		if err == nil {
			t.Errorf("Parsing empty string should trigger an Error, but it did not.")
		}
	})
}

func TestNewPropertyFromString(t *testing.T) {

	t.Run("Parse valid property string", func(t *testing.T) {
		prop := "test.key=test value"
		expected := Property{
			key:     "test.key",
			value:   "test value",
			comment: "",
		}

		p, err := newPropertyFromString(prop, "")

		if err != nil {
			t.Errorf("Parsing valid Property string should not result an error: %v", err)
		}

		if !p.Equal(expected) {
			t.Errorf("Returned and the expected Property objects mismatch. Expected %v, got %v", expected, p)
		}
	})

	t.Run("Parse valid property string with escaped key", func(t *testing.T) {
		prop := "test\\:key=test value"
		expected := Property{
			key:   "test:key",
			value: "test value",
		}

		p, err := newPropertyFromString(prop, "")

		if err != nil {
			t.Errorf("Parsing valid Property string should not result an error: %v", err)
		}

		if !p.Equal(expected) {
			t.Errorf("Returned and the expected Property objects mismatch. Expected %v, got %v", expected, p)
		}
	})

	t.Run("Parse invalid property string", func(t *testing.T) {
		prop := "test.key.test.value"

		_, err := newPropertyFromString(prop, "")

		if err == nil {
			t.Errorf("Parsing invalid Property string should trigger an error!")
		}
	})
}
