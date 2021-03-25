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

	. "github.com/onsi/gomega"
)

const (
	testKey   = "test.key"
	testValue = "test value"
)

func TestProperty(t *testing.T) {
	g := NewGomegaWithT(t)

	k := testKey
	v := testValue

	p := Property{
		key:   k,
		value: v,
	}

	g.Expect(p.Key()).Should(Equal(k))
	g.Expect(p.Value()).Should(Equal(v))
}

func TestPropertyComment(t *testing.T) {
	g := NewGomegaWithT(t)

	k := testKey
	v := testValue
	c := "this is a comment"

	p := Property{
		key:     k,
		value:   v,
		comment: c,
	}

	g.Expect(p.Comment()).Should(Equal(c))
}

func TestPropertyString(t *testing.T) {
	g := NewGomegaWithT(t)

	k := testKey
	v := testValue

	p := Property{
		key:   k,
		value: v,
	}

	expected := fmt.Sprintf("%s=%s", k, v)
	g.Expect(fmt.Sprint(p)).Should(Equal(expected))
}

func TestPropertyInt(t *testing.T) {

	validProp := Property{
		key:   testKey,
		value: "100",
	}

	invalidProp := Property{
		key:   testKey,
		value: "not an integer",
	}

	t.Run("Parsing valid value to integer should succeed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		i, err := validProp.Int()
		g.Expect(err).Should(Succeed())
		g.Expect(i).Should(Equal(int64(100)))
	})

	t.Run("Parsing invalid value to integer should fail", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := invalidProp.Int()
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestPropertyFloat(t *testing.T) {

	validProp := Property{
		key:   testKey,
		value: "100.00",
	}

	invalidProp := Property{
		key:   testKey,
		value: "not a float",
	}

	t.Run("Parsing valid value to float should succeed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		f, err := validProp.Float()
		g.Expect(err).Should(Succeed())
		g.Expect(f).Should(Equal(100.00))
	})

	t.Run("Parsing invalid value to float should fail", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := invalidProp.Float()
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestPropertyBool(t *testing.T) {

	trueProp := Property{
		key:   testKey,
		value: "true",
	}

	falseProp := Property{
		key:   testKey,
		value: "FALSE",
	}

	invalidProp := Property{
		key:   testKey,
		value: "not a boolean",
	}

	t.Run("Parsing true to boolean should succeed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		b, err := trueProp.Bool()
		g.Expect(err).Should(Succeed())
		g.Expect(b).Should(BeTrue())
	})

	t.Run("Parsing false to boolean should succeed", func(t *testing.T) {
		g := NewGomegaWithT(t)

		b, err := falseProp.Bool()
		g.Expect(err).Should(Succeed())
		g.Expect(b).Should(BeFalse())
	})

	t.Run("Parsing invalid value to boolean should fail", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := invalidProp.Bool()
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestPropertyList(t *testing.T) {

	validProp := Property{
		key:   testKey,
		value: "test item1,test item2,test item3",
	}

	notListProp := Property{
		key:   testKey,
		value: "not a list value",
	}

	t.Run("Parsing Property with list value should return a slice of strings", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := []string{"test item1", "test item2", "test item3"}

		l, _ := validProp.List()
		g.Expect(l).Should(Equal(expected))
	})

	t.Run("Parsing Property with non-list value should return a slice of string with a single element", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := []string{"not a list value"}

		l, _ := notListProp.List()
		g.Expect(l).Should(Equal(expected))
	})
}

func TestPropertyGetByType(t *testing.T) {

	t.Run("Converting Property value to Int", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "128",
		}
		expected := int64(128)

		v, _ := prop.GetByType(Int)
		g.Expect(v).Should(Equal(expected))
	})

	t.Run("Converting Property value to Float", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "128.9",
		}
		expected := 128.9

		v, _ := prop.GetByType(Float)
		g.Expect(v).Should(Equal(expected))
	})

	t.Run("Converting Property value to String", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "test item",
		}
		expected := "test item"

		v, _ := prop.GetByType(String)
		g.Expect(v).Should(Equal(expected))
	})

	t.Run("Converting Property value to String", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "true",
		}
		expected := true

		v, _ := prop.GetByType(Bool)
		g.Expect(v).Should(Equal(expected))
	})

	t.Run("Converting Property value to List", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "item1,item2,item3",
		}
		expected := []string{"item1", "item2", "item3"}

		v, _ := prop.GetByType(List)
		g.Expect(v).Should(Equal(expected))
	})

	t.Run("Converting Property value to List", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := Property{
			key:   testKey,
			value: "item1,item2,item3",
		}

		_, err := prop.GetByType(Invalid)
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestPropertySet(t *testing.T) {

	t.Run("Set with nil interface as value", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{}

		err := prop.set("key", nil, "")
		g.Expect(err).Should(HaveOccurred())
	})

	t.Run("Set with unsupported slice value", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{}

		err := prop.set("key", []int{0, 1, 2}, "")
		g.Expect(err).Should(HaveOccurred())
	})

	t.Run("Set with unsupported value", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{}

		err := prop.set("key", map[int]int{0: 0, 1: 1, 2: 2}, "")
		g.Expect(err).Should(HaveOccurred())
	})
}

func TestPropertyIsValid(t *testing.T) {

	t.Run("Empty key and value fields", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{}
		g.Expect(prop.IsValid()).Should(BeFalse())
	})

	t.Run("Non-empty key field", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{testKey, "", ""}
		g.Expect(prop.IsValid()).Should(BeTrue())
	})
}

func TestPropertyIsEmpty(t *testing.T) {

	t.Run("Empty key and value fields", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{}
		g.Expect(prop.IsEmpty()).Should(BeTrue())
	})

	t.Run("Empty key field", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{testKey, "", ""}
		g.Expect(prop.IsEmpty()).Should(BeTrue())
	})

	t.Run("Non-empty key and value fields", func(t *testing.T) {
		g := NewGomegaWithT(t)

		prop := &Property{testKey, "test.value", ""}
		g.Expect(prop.IsEmpty()).Should(BeFalse())
	})
}
