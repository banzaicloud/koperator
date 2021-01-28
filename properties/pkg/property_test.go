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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestProperty(t *testing.T) {
	k := "test.key"
	v := "test value"

	p := Property{
		key:   k,
		value: v,
	}

	if p.Key() != k || p.Value() != v {
		t.Errorf("Property name and/or value are not set correctly. Expected: %s=%s, got %s=%s",
			k, v, p.Key(), p.Value())
	}

}

func TestPropertyComment(t *testing.T) {
	k := "test.key"
	v := "test value"
	c := "this is a comment"

	p := Property{
		key:     k,
		value:   v,
		comment: c,
	}

	if p.Comment() != c {
		t.Errorf("Comment for Property is not returned correctly. Expected: %q, got %q",
			c, p.Comment())
	}
}

func TestPropertyString(t *testing.T) {
	k := "test.key"
	v := "test value"

	p := Property{
		key:   k,
		value: v,
	}

	s := fmt.Sprint(p)
	expected := fmt.Sprintf("%s=%s", k, v)
	if s != expected {
		t.Errorf("Property name and/or value are not set correctly. Expected: %q, got %q",
			s, expected)
	}
}

func TestPropertyInt(t *testing.T) {

	validProp := Property{
		key:   "test.key",
		value: "100",
	}

	invalidProp := Property{
		key:   "test.key",
		value: "not an integer",
	}

	t.Run("Parsing valid value to integer should succeed", func(t *testing.T) {
		if _, err := validProp.Int(); err != nil {
			t.Errorf("Coverting the value of Property to integer resulted an error: %s", err)
		}
	})

	t.Run("Parsing valid value to integer should succeed", func(t *testing.T) {
		if i, _ := validProp.Int(); fmt.Sprintf("%d", i) != validProp.value {
			t.Errorf("Coverting the value of Property to integer resulted a value mismatch. Expected: %s, got %d",
				validProp.value, i)
		}
	})

	t.Run("Parsing invalid value to integer should fail", func(t *testing.T) {
		if _, err := invalidProp.Int(); err == nil {
			t.Errorf("Coverting the value of Property to integer resulted an error: %s", err)
		}
	})
}

func TestPropertyFloat(t *testing.T) {

	validProp := Property{
		key:   "test.key",
		value: "100.00",
	}

	invalidProp := Property{
		key:   "test.key",
		value: "not a float",
	}

	t.Run("Parsing valid value to float should succeed", func(t *testing.T) {
		if _, err := validProp.Float(); err != nil {
			t.Errorf("Coverting the value of Property to float resulted an error: %s", err)
		}
	})

	t.Run("Parsing valid value to float should succeed", func(t *testing.T) {
		if f, _ := validProp.Float(); f != 100.00 {
			t.Errorf("Coverting the value of Property to float resulted a value mismatch. Expected: %s, got %f",
				validProp.value, f)
		}
	})

	t.Run("Parsing invalid value to float should fail", func(t *testing.T) {
		if _, err := invalidProp.Float(); err == nil {
			t.Errorf("Coverting the value of Property to float resulted an error: %s", err)
		}
	})
}

func TestPropertyBool(t *testing.T) {

	trueProp := Property{
		key:   "test.key",
		value: "true",
	}

	falseProp := Property{
		key:   "test.key",
		value: "FALSE",
	}

	invalidProp := Property{
		key:   "test.key",
		value: "not a boolean",
	}

	t.Run("Parsing valid value to boolean should succeed", func(t *testing.T) {
		b, err := trueProp.Bool()

		if err != nil {
			t.Errorf("Coverting the value of Property to boolean resulted an error: %s", err)
		}

		if !b {
			t.Errorf("Coverting the value of Property to float resulted a value mismatch. Expected: true, got %v", b)
		}
	})

	t.Run("Parsing valid value to boolean should succeed", func(t *testing.T) {
		b, err := falseProp.Bool()

		if err != nil {
			t.Errorf("Coverting the value of Property to boolean resulted an error: %s", err)
		}

		if b {
			t.Errorf("Coverting the value of Property to float resulted a value mismatch. Expected: false, got %v", b)
		}
	})

	t.Run("Parsing invalid value to boolean should fail", func(t *testing.T) {
		if _, err := invalidProp.Bool(); err == nil {
			t.Errorf("Coverting invalid value of Property to boolean did not result an error")
		}
	})
}

func TestPropertyList(t *testing.T) {

	validProp := Property{
		key:   "test.key",
		value: "test item1,test item2,test item3",
	}

	notListProp := Property{
		key:   "test.key",
		value: "not a list value",
	}

	t.Run("Parsing Property with list value should return a slice of strings", func(t *testing.T) {
		expected := []string{"test item1", "test item2", "test item3"}
		if l, _ := validProp.List(); !cmp.Equal(l, expected) {
			t.Errorf("Mismatch in the expected and the resulted slices. Expected %q, got %q", expected, l)
		}
	})

	t.Run("Parsing Property with non-list value should return a slice of string with a single element", func(t *testing.T) {
		expected := []string{"not a list value"}
		if l, _ := notListProp.List(); !cmp.Equal(l, expected) {
			t.Errorf("Mismatch in the expected and the resulted slices. Expected %q, got %q", expected, l)
		}
	})
}

func TestPropertyGetByType(t *testing.T) {

	t.Run("Converting Property value to Int", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "128",
		}

		expected := int64(128)
		if v, _ := prop.GetByType(Int); v != expected {
			t.Errorf("Value mismatch. Expected %v, got %v", expected, v)
		}
	})

	t.Run("Converting Property value to Float", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "128.9",
		}

		expected := 128.9
		if v, _ := prop.GetByType(Float); v != expected {
			t.Errorf("Value mismatch. Expected %v, got %v", expected, v)
		}
	})

	t.Run("Converting Property value to String", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "test item",
		}

		expected := "test item"
		if v, _ := prop.GetByType(String); v != expected {
			t.Errorf("Value mismatch. Expected %v, got %v", expected, v)
		}
	})

	t.Run("Converting Property value to String", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "true",
		}

		expected := true
		if v, _ := prop.GetByType(Bool); v != expected {
			t.Errorf("Value mismatch. Expected %v, got %v", expected, v)
		}
	})

	t.Run("Converting Property value to List", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "item1,item2,item3",
		}

		expected := []string{"item1", "item2", "item3"}
		if v, _ := prop.GetByType(List); !cmp.Equal(v, expected) {
			t.Errorf("Value mismatch. Expected %v, got %v", expected, v)
		}
	})

	t.Run("Converting Property value to List", func(t *testing.T) {
		prop := Property{
			key:   "test.key",
			value: "item1,item2,item3",
		}

		if _, err := prop.GetByType(Invalid); err == nil {
			t.Errorf("Calling ToType with Invalid PropertyType should result an error.")
		}
	})
}

func TestPropertySet(t *testing.T) {

	t.Run("Set with nil interface as value", func(t *testing.T) {
		prop := &Property{}

		if err := prop.set("key", nil, ""); err == nil {
			t.Errorf("Updating Property with nil interface value should return an error!")
		}
	})

	t.Run("Set with unsupported slice value", func(t *testing.T) {
		prop := &Property{}

		if err := prop.set("key", []int{0, 1, 2}, ""); err == nil {
			t.Errorf("Updating Property with unsupported slice should return an error!")
		}
	})

	t.Run("Set with unsupported value", func(t *testing.T) {
		prop := &Property{}

		if err := prop.set("key", map[int]int{0: 0, 1: 1, 2: 2}, ""); err == nil {
			t.Errorf("Updating Property with unsupported type should return an error!")
		}
	})
}
