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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewProperties(t *testing.T) {
	p := NewProperties()

	if len(p.keys) != 0 && len(p.properties) != 0 {
		t.Errorf("Newly created Properties is not empty")
	}
}

func TestProperties_Get(t *testing.T) {
	k := "test.key"
	v := "test.value"
	c := "this is a comment"

	p := NewProperties()
	p.put(Property{k, v, c})

	t.Run("Get existing Property", func(t *testing.T) {
		if _, found := p.Get(k); !found {
			t.Errorf("Existing Property is not found. Expected: %v, got %v", found, !found)
		}
	})

	t.Run("Get nonexistent Property", func(t *testing.T) {
		if _, found := p.Get(k + "-"); found {
			t.Errorf("None existing Property found. Expected: %v, got %v", !found, found)
		}
	})

	t.Run("Expected Property is returned", func(t *testing.T) {
		expectedProp := Property{
			key:     k,
			value:   v,
			comment: c,
		}

		if prop, found := p.Get(k); !found || !prop.Equal(expectedProp) {
			t.Error("Mismatch between the returned and the expected Property")
		}
	})
}

func TestProperties_Keys(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	t.Run("Get list of keys in Properties", func(t *testing.T) {
		expectedKeys := []string{
			"test.key",
			"test.key2",
		}

		if keys := p.Keys(); !cmp.Equal(keys, expectedKeys) {
			t.Errorf("Mismatch between return and expected list of keys. Expected: %q, got %q", expectedKeys, keys)
		}
	})

	t.Run("Get list of keys in Properties after adding a new Property", func(t *testing.T) {
		expectedKeys := []string{
			"test.key",
			"test.key2",
			"test.key3",
		}

		p.put(Property{"test.key3", "test.value3", ""})

		if keys := p.Keys(); !cmp.Equal(keys, expectedKeys) {
			t.Errorf("Mismatch between return and expected list of keys. Expected: %q, got %q", expectedKeys, keys)
		}
	})

	t.Run("Get list of keys in Properties after deleting a Property", func(t *testing.T) {
		expectedKeys := []string{
			"test.key2",
			"test.key3",
		}

		p.Delete("test.key")

		if keys := p.Keys(); !cmp.Equal(keys, expectedKeys) {
			t.Errorf("Mismatch in list of keys after deletion. Expected: %q, got %q", expectedKeys, keys)
		}
	})
}

func TestProperties_Set(t *testing.T) {

	p := NewProperties()

	t.Run("Add new Property with string value to Properties", func(t *testing.T) {
		expectedProperty := Property{
			key:     "test.key3",
			value:   "test.value3",
			comment: "this is a comment line",
		}

		err := p.Set(expectedProperty.key, expectedProperty.value, expectedProperty.comment)

		if err != nil {
			t.Error("Adding a new Property should not result an error.")
		}

		prop, found := p.Get(expectedProperty.key)

		if !found {
			t.Error("Newly added Property is not found.")
		}

		if !prop.Equal(expectedProperty) {
			t.Errorf("Mismatch between return and expected list of keys. Expected: %q, got %q", prop, expectedProperty)
		}
	})

	t.Run("Add new Property with integer value to Properties", func(t *testing.T) {
		expectedProperty := Property{
			key:   "test.key3",
			value: "100",
		}

		err := p.Set(expectedProperty.key, 100, "")

		if err != nil {
			t.Error("Adding a new property should not result an error.")
		}

		prop, found := p.Get(expectedProperty.key)

		if !found {
			t.Errorf("Newly added Property is not found.")
		}

		if !prop.Equal(expectedProperty) {
			t.Errorf("Mismatch between return and expected list of keys. Expected: %v, got %v", prop, expectedProperty)
		}
	})
}

func TestProperties_Delete(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	t.Run("Delete existing Property from Properties", func(t *testing.T) {

		p.Delete("test.key2")

		if _, found := p.Get("test.key2"); found {
			t.Errorf("Deleted Property should not be found.")
		}
	})
}

func TestProperties_Len(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	t.Run("Get number of Property in Properties", func(t *testing.T) {
		expected := 2

		if p.Len() != expected {
			t.Errorf("Incorrect number of Property. Expected: %v, got %v", expected, p.Len())
		}
	})
}

func TestProperties_Merge(t *testing.T) {

	t.Run("Merge from nil Properties", func(t *testing.T) {
		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p1",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p1",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
		}

		dst.Merge(nil)

		if !dst.Equal(expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				expectedProperties, dst)
		}
	})

	p1 := NewProperties()
	p1.put(Property{"test.key", "p1", "this is a comment line"})
	p1.put(Property{"test.key2", "p1", "this is a comment line"})

	p2 := NewProperties()
	p2.put(Property{"test.key2", "p2", "this is a comment line"})
	p2.put(Property{"test.key3", "p2", "this is a comment line"})

	t.Run("Merge two Properties", func(t *testing.T) {
		expectedProperties := Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p1",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p2",
				},
				"test.key3": {
					key:   "test.key3",
					value: "p2",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
				"test.key3": {key: "test.key3", index: 2},
			},
		}

		p1.Merge(p2)

		if !p1.Equal(&expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				&expectedProperties, p1)
		}
	})
}

func TestProperties_MergeDefaults(t *testing.T) {

	t.Run("Merge defaults from nil Properties", func(t *testing.T) {
		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p1",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p1",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
		}

		dst.MergeDefaults(nil)

		if !dst.Equal(expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				expectedProperties, dst)
		}
	})

	t.Run("Merge defaults into empty Properties", func(t *testing.T) {
		dst := NewProperties()

		src := NewProperties()
		src.put(Property{"test.key", "p2", "this is a comment line"})
		src.put(Property{"test.key2", "p2", "this is a comment line"})
		src.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p2",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p2",
				},
				"test.key3": {
					key:   "test.key3",
					value: "p2",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
				"test.key3": {key: "test.key3", index: 2},
			},
		}

		dst.MergeDefaults(src)

		if !dst.Equal(expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				expectedProperties, dst)
		}
	})

	t.Run("Merge into non-empty Properties", func(t *testing.T) {
		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		src := NewProperties()
		src.put(Property{"test.key2", "p2", "this is a comment line"})
		src.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p1",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p1",
				},
				"test.key3": {
					key:   "test.key3",
					value: "p2",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
				"test.key3": {key: "test.key3", index: 2},
			},
		}

		dst.MergeDefaults(src)

		if !dst.Equal(expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				expectedProperties, dst)
		}
	})

	t.Run("No update after merge defaults", func(t *testing.T) {
		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		src := NewProperties()
		src.put(Property{"test.key", "p2", "this is a comment line"})
		src.put(Property{"test.key2", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "p1",
				},
				"test.key2": {
					key:   "test.key2",
					value: "p1",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
		}

		dst.MergeDefaults(src)

		if !dst.Equal(expectedProperties) {
			t.Errorf("Mismatch in expected and returned Properties!\nExpected: %q\nGot: %q\n",
				expectedProperties, dst)
		}
	})
}

func TestProperties_Equal(t *testing.T) {
	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("Nil", func(t *testing.T) {
		if p.Equal(nil) {
			t.Errorf("Comparing valid and malformed Properties must not be equal!")
		}
	})

	t.Run("Malformed Properties", func(t *testing.T) {
		expected := Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "test.value",
				},
				"test.key2": {
					key:   "test.key2",
					value: "test.value2",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
				"test.key3": {key: "test.key3", index: 2},
			},
			nextKeyIndex: 3,
		}

		if p.Equal(&expected) {
			t.Errorf("Comparing valid and malformed Properties must not be equal!")
		}
	})

	t.Run("Mismatch in number of keys", func(t *testing.T) {
		expected := Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "test.value",
				},
				"test.key4": {
					key:   "test.key4",
					value: "test.value4",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key4": {key: "test.key4", index: 1},
			},
			nextKeyIndex: 2,
		}

		if p.Equal(&expected) {
			t.Errorf("Comparing Properties two objects with different set of keys must not be equal!")
		}
	})

	t.Run("Mismatch in keys", func(t *testing.T) {
		expected := Properties{
			properties: map[string]Property{
				"test.key": {
					key:   "test.key",
					value: "test.value",
				},
				"test.key4": {
					key:   "test.key4",
					value: "test.value4",
				},
				"test.key5": {
					key:   "test.key5",
					value: "test.value5",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key4": {key: "test.key4", index: 1},
				"test.key5": {key: "test.key5", index: 2},
			},
			nextKeyIndex: 3,
		}

		if p.Equal(&expected) {
			t.Errorf("Comparing Properties two objects with different set of keys must not be equal!")
		}
	})

	t.Run("Property for key not equal", func(t *testing.T) {
		expected := Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "test.value",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "test.value",
					comment: "this is a comment line",
				},
				"test.key3": {
					key:     "test.key3",
					value:   "test.value",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
				"test.key3": {key: "test.key3", index: 2},
			},
			nextKeyIndex: 3,
		}

		if p.Equal(&expected) {
			t.Errorf("Comparing Properties two objects with same set of keys, but different Property objects must not be equal!")
		}
	})
}

func TestProperties_String(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("Convert to string", func(t *testing.T) {
		expectedString := `test.key=test.value
test.key2=test.value2
test.key3=test.value3
`

		if !cmp.Equal(fmt.Sprint(p), expectedString) {
			t.Errorf("Mismatch in result of converting Properties to string. Expected: %v, got %v", expectedString, fmt.Sprint(p))
		}
	})
}

func TestProperties_MarshalJSON(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("To JSON", func(t *testing.T) {
		expectedString := `{"properties":{"test.key":"test.value","test.key2":"test.value2","test.key3":"test.value3"}}`
		pJson, err := json.Marshal(&p)

		if err != nil {
			t.Errorf("Converting Properties to JSON resulted an error: %s", err)
		}

		if !cmp.Equal(string(pJson), expectedString) {
			t.Errorf("Mismatch in result of converting Properties to JSON. Expected: %v, got %v", expectedString, string(pJson))
		}
	})
}

func TestKeyIndexList(t *testing.T) {

	kIdxList := keyIndexList{
		keyIndex{"test.key11", 11},
		keyIndex{"test.key2", 2},
		keyIndex{"test.key5", 5},
		keyIndex{"test.key35", 35},
	}

	t.Run("Swap", func(t *testing.T) {
		expected := keyIndexList{
			keyIndex{"test.key35", 35},
			keyIndex{"test.key2", 2},
			keyIndex{"test.key5", 5},
			keyIndex{"test.key11", 11},
		}

		kIdxList.Swap(0,3)

		if !reflect.DeepEqual(kIdxList, expected) {
			t.Errorf("Mismatch in result of swaping items keyIndexList. Expected: %v, got %v", expected, kIdxList)
		}
	})
}
