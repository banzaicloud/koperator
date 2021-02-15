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
	"sort"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewProperties(t *testing.T) {
	g := NewGomegaWithT(t)

	p := NewProperties()
	g.Expect(p.keys).Should(BeEmpty())
	g.Expect(p.properties).Should(BeEmpty())
}

func TestProperties_Get(t *testing.T) {
	k := "test.key"
	v := "test.value"
	c := "this is a comment"

	p := NewProperties()
	p.put(Property{k, v, c})

	t.Run("Get existing Property", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, found := p.Get(k)
		g.Expect(found).Should(BeTrue())
	})

	t.Run("Get nonexistent Property", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, found := p.Get(k + "-")
		g.Expect(found).Should(BeFalse())
	})

	t.Run("Expected Property is returned", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProp := Property{
			key:     k,
			value:   v,
			comment: c,
		}

		prop, found := p.Get(k)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(Equal(expectedProp))
	})
}

func TestProperties_Keys(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	t.Run("Get list of keys in Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedKeys := []string{
			"test.key",
			"test.key2",
		}

		keys := p.Keys()
		g.Expect(keys).Should(Equal(expectedKeys))
	})

	t.Run("Get list of keys in Properties after adding a new Property", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedKeys := []string{
			"test.key",
			"test.key2",
			"test.key3",
		}

		p.put(Property{"test.key3", "test.value3", ""})

		keys := p.Keys()
		g.Expect(keys).Should(Equal(expectedKeys))
	})

	t.Run("Get list of keys in Properties after deleting a Property", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedKeys := []string{
			"test.key2",
			"test.key3",
		}

		p.Delete("test.key")

		keys := p.Keys()
		g.Expect(keys).Should(Equal(expectedKeys))
	})
}

func TestProperties_Set(t *testing.T) {

	p := NewProperties()

	t.Run("Add new Property with string value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:   "test.key3",
			value: "test.value3",
		}

		err := p.Set(expectedProperty.key, expectedProperty.value)
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})

	t.Run("Add new Property with integer value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:   "test.key3",
			value: "100",
		}

		err := p.Set(expectedProperty.key, 100)
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})

	t.Run("Add new Property with string slice value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:   "test.key3",
			value: "value1,value2,value3",
		}

		err := p.Set(expectedProperty.key, []string{"value1", "value2", "value3"})
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})
}

func TestProperties_SetWithComment(t *testing.T) {

	p := NewProperties()

	t.Run("Add new Property with string value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:     "test.key3",
			value:   "test.value3",
			comment: "this is a comment line",
		}

		err := p.SetWithComment(expectedProperty.key, expectedProperty.value, expectedProperty.comment)
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})

	t.Run("Add new Property with integer value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:     "test.key3",
			value:   "100",
			comment: "this is a comment line",
		}

		err := p.SetWithComment(expectedProperty.key, 100, expectedProperty.comment)
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})

	t.Run("Add new Property with string slice value to Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperty := Property{
			key:     "test.key3",
			value:   "value1,value2,value3",
			comment: "this is a comment line",
		}

		err := p.SetWithComment(expectedProperty.key, []string{"value1", "value2", "value3"}, expectedProperty.comment)
		g.Expect(err).Should(BeNil())

		prop, found := p.Get(expectedProperty.key)
		g.Expect(found).Should(BeTrue())
		g.Expect(prop).Should(BeEquivalentTo(expectedProperty))
	})
}

func TestProperties_Delete(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	t.Run("Delete existing Property from Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p.Delete("test.key2")

		_, found := p.Get("test.key2")
		g.Expect(found).Should(BeFalse())
	})
}

func TestProperties_Len(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})

	g := NewGomegaWithT(t)
	g.Expect(p.Len()).Should(Equal(2))
}

func TestProperties_Merge(t *testing.T) {

	t.Run("Merge from nil Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p1",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
			nextKeyIndex: 2,
		}

		dst.Merge(nil)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})

	t.Run("Merge two Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p2",
					comment: "this is a comment line",
				},
				"test.key3": {
					key:     "test.key3",
					value:   "p2",
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

		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		src := NewProperties()
		src.put(Property{"test.key2", "p2", "this is a comment line"})
		src.put(Property{"test.key3", "p2", "this is a comment line"})

		dst.Merge(src)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})
}

func TestProperties_MergeDefaults(t *testing.T) {

	t.Run("Merge defaults from nil Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p1",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
			nextKeyIndex: 2,
		}

		dst.MergeDefaults(nil)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})

	t.Run("Merge defaults into empty Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		dst := NewProperties()

		src := NewProperties()
		src.put(Property{"test.key", "p2", "this is a comment line"})
		src.put(Property{"test.key2", "p2", "this is a comment line"})
		src.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p2",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p2",
					comment: "this is a comment line",
				},
				"test.key3": {
					key:     "test.key3",
					value:   "p2",
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

		dst.MergeDefaults(src)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})

	t.Run("Merge into non-empty Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		src := NewProperties()
		src.put(Property{"test.key2", "p2", "this is a comment line"})
		src.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key3": {
					key:     "test.key3",
					value:   "p2",
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

		dst.MergeDefaults(src)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})

	t.Run("No update after merge defaults", func(t *testing.T) {
		g := NewGomegaWithT(t)

		dst := NewProperties()
		dst.put(Property{"test.key", "p1", "this is a comment line"})
		dst.put(Property{"test.key2", "p1", "this is a comment line"})

		src := NewProperties()
		src.put(Property{"test.key", "p2", "this is a comment line"})
		src.put(Property{"test.key2", "p2", "this is a comment line"})

		expectedProperties := &Properties{
			properties: map[string]Property{
				"test.key": {
					key:     "test.key",
					value:   "p1",
					comment: "this is a comment line",
				},
				"test.key2": {
					key:     "test.key2",
					value:   "p1",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"test.key":  {key: "test.key", index: 0},
				"test.key2": {key: "test.key2", index: 1},
			},
			nextKeyIndex: 2,
		}

		dst.MergeDefaults(src)
		g.Expect(dst).Should(BeEquivalentTo(expectedProperties))
	})
}

func TestProperties_Equal(t *testing.T) {
	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("Nil", func(t *testing.T) {
		g := NewGomegaWithT(t)
		g.Expect(p.Equal(nil)).Should(BeFalse())
	})

	t.Run("Malformed Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &Properties{
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

		g.Expect(p.Equal(expected)).ShouldNot(BeTrue())
	})

	t.Run("Mismatch in number of keys", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &Properties{
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

		g.Expect(p.Equal(expected)).ShouldNot(BeTrue())
	})

	t.Run("Mismatch in keys", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &Properties{
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

		g.Expect(p.Equal(expected)).ShouldNot(BeTrue())
	})

	t.Run("Property for key not equal", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &Properties{
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

		g.Expect(p.Equal(expected)).Should(BeFalse())
	})
}

func TestProperties_Sort(t *testing.T) {

	t.Run("Reindex Properties to sort keys alphabetically", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p := NewProperties()
		p.put(Property{"b.key", "b", "this is a comment line"})
		p.put(Property{"c.key", "c", "this is a comment line"})
		p.put(Property{"a.key", "a", "this is a comment line"})

		expected := &Properties{
			properties: map[string]Property{
				"a.key": {
					key:     "a.key",
					value:   "a",
					comment: "this is a comment line",
				},
				"b.key": {
					key:     "b.key",
					value:   "b",
					comment: "this is a comment line",
				},
				"c.key": {
					key:     "c.key",
					value:   "c",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"a.key": {key: "a.key", index: 0},
				"b.key": {key: "b.key", index: 1},
				"c.key": {key: "c.key", index: 2},
			},
			nextKeyIndex: 3,
		}

		p.Sort()
		g.Expect(p).Should(BeEquivalentTo(expected))
	})

	t.Run("Sort already sorted Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p := NewProperties()
		p.put(Property{"a.key", "a", "this is a comment line"})
		p.put(Property{"b.key", "b", "this is a comment line"})
		p.put(Property{"c.key", "c", "this is a comment line"})

		expected := &Properties{
			properties: map[string]Property{
				"a.key": {
					key:     "a.key",
					value:   "a",
					comment: "this is a comment line",
				},
				"b.key": {
					key:     "b.key",
					value:   "b",
					comment: "this is a comment line",
				},
				"c.key": {
					key:     "c.key",
					value:   "c",
					comment: "this is a comment line",
				},
			},
			keys: map[string]keyIndex{
				"a.key": {key: "a.key", index: 0},
				"b.key": {key: "b.key", index: 1},
				"c.key": {key: "c.key", index: 2},
			},
			nextKeyIndex: 3,
		}

		p.Sort()
		g.Expect(p).Should(BeEquivalentTo(expected))
	})
}

func TestProperties_String(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("Convert to string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedString := `test.key=test.value
test.key2=test.value2
test.key3=test.value3
`
		g.Expect(fmt.Sprint(p)).Should(Equal(expectedString))
	})
}

func TestProperties_MarshalJSON(t *testing.T) {

	p := NewProperties()
	p.put(Property{"test.key", "test.value", "this is a comment line"})
	p.put(Property{"test.key2", "test.value2", "this is a comment line"})
	p.put(Property{"test.key3", "test.value3", "this is a comment line"})

	t.Run("To JSON", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expectedString := []byte(`{"properties":{"test.key":"test.value","test.key2":"test.value2","test.key3":"test.value3"}}`)

		pJson, err := json.Marshal(&p)
		g.Expect(err).Should(BeNil())
		g.Expect(pJson).Should(Equal(expectedString))
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
		g := NewGomegaWithT(t)

		expected := keyIndexList{
			keyIndex{"test.key35", 35},
			keyIndex{"test.key2", 2},
			keyIndex{"test.key5", 5},
			keyIndex{"test.key11", 11},
		}

		kIdxList.Swap(0, 3)
		g.Expect(kIdxList).Should(Equal(expected))
	})
}

func TestProperties_Diff(t *testing.T) {

	t.Run("Difference when both Properties are empty", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p2 := NewProperties()

		expectedDiffMap := DiffResult{}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		expectedKeys := make([]string, 0)
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})

	t.Run("Difference when left Properties is empty", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p2 := NewProperties()

		p2.put(Property{"test.key", "p2", "this is a comment line"})
		p2.put(Property{"test.key2", "p2", "this is a comment line"})
		p2.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{
			"test.key": [2]Property{
				{},
				{"test.key", "p2", "this is a comment line"},
			},
			"test.key2": [2]Property{
				{},
				{"test.key2", "p2", "this is a comment line"},
			},
			"test.key3": [2]Property{
				{},
				{"test.key3", "p2", "this is a comment line"},
			},
		}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := []string{"test.key", "test.key2", "test.key3"}
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})

	t.Run("Difference when some keys are missing from the left Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p1.put(Property{"test.key2", "p1", "this is a comment line"})

		p2 := NewProperties()
		p2.put(Property{"test.key", "p2", "this is a comment line"})
		p2.put(Property{"test.key2", "p2", "this is a comment line"})
		p2.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{
			"test.key": [2]Property{
				{},
				{"test.key", "p2", "this is a comment line"},
			},
			"test.key2": [2]Property{
				{"test.key2", "p1", "this is a comment line"},
				{"test.key2", "p2", "this is a comment line"},
			},
			"test.key3": [2]Property{
				{},
				{"test.key3", "p2", "this is a comment line"},
			},
		}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := []string{"test.key", "test.key2", "test.key3"}
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})

	t.Run("Difference when some keys are missing from the right Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p1.put(Property{"test.key", "p1", "this is a comment line"})
		p1.put(Property{"test.key2", "p1", "this is a comment line"})
		p1.put(Property{"test.key3", "p1", "this is a comment line"})

		p2 := NewProperties()
		p2.put(Property{"test.key", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{
			"test.key": [2]Property{
				{"test.key", "p1", "this is a comment line"},
				{"test.key", "p2", "this is a comment line"},
			},
			"test.key2": [2]Property{
				{"test.key2", "p1", "this is a comment line"},
				{},
			},
			"test.key3": [2]Property{
				{"test.key3", "p1", "this is a comment line"},
				{},
			},
		}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := []string{"test.key", "test.key2", "test.key3"}
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})

	t.Run("Difference identical Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p1.put(Property{"test.key", "p2", "this is a comment line"})
		p1.put(Property{"test.key2", "p2", "this is a comment line"})
		p1.put(Property{"test.key3", "p2", "this is a comment line"})

		p2 := NewProperties()
		p2.put(Property{"test.key", "p2", "this is a comment line"})
		p2.put(Property{"test.key2", "p2", "this is a comment line"})
		p2.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := make([]string, 0)
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))

	})

	t.Run("Difference nil Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p1.put(Property{"test.key", "p2", "this is a comment line"})
		p1.put(Property{"test.key2", "p2", "this is a comment line"})
		p1.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{}

		diff := p1.Diff(nil)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := make([]string, 0)
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})

	t.Run("Difference of nearly identical Properties", func(t *testing.T) {
		g := NewGomegaWithT(t)

		p1 := NewProperties()
		p1.put(Property{"test.key", "p2", "this is a comment line"})
		p1.put(Property{"test.key2", "p1", "this is a comment line"})
		p1.put(Property{"test.key3", "p2", "this is a comment line"})

		p2 := NewProperties()
		p2.put(Property{"test.key", "p2", "this is a comment line"})
		p2.put(Property{"test.key2", "p2", "this is a comment line"})
		p2.put(Property{"test.key3", "p2", "this is a comment line"})

		expectedDiffMap := DiffResult{
			"test.key2": [2]Property{
				{"test.key2", "p1", "this is a comment line"},
				{"test.key2", "p2", "this is a comment line"},
			},
		}

		diff := p1.Diff(p2)
		g.Expect(diff).Should(Equal(expectedDiffMap))

		keys := diff.Keys()
		sort.Strings(keys)
		expectedKeys := []string{"test.key2"}
		g.Expect(keys).Should(Equal(expectedKeys))

		g.Expect(len(diff)).Should(Equal(len(expectedDiffMap)))
	})
}

func TestProperties_DiffMap(t *testing.T) {

	diffMap := DiffResult{
		"test.key": [2]Property{
			{"test.key", "p1", "this is a comment line"},
			{"test.key", "p2", "this is a comment line"},
		},
		"test.key2": [2]Property{
			{"test.key2", "p1", "this is a comment line"},
			{},
		},
		"test.key3": [2]Property{
			{"test.key3", "p1", "this is a comment line"},
			{},
		},
	}

	t.Run("Keys", func(t *testing.T) {
		g := NewGomegaWithT(t)

		keys := diffMap.Keys()
		sort.Strings(keys)
		expectedKeys := []string{"test.key", "test.key2", "test.key3"}
		g.Expect(keys).Should(Equal(expectedKeys))
	})
}
