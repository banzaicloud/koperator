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

package utils

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestStructTag(t *testing.T) {
	t.Run("Skip with empty key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		st := &StructTag{
			Key:       "",
			OmitEmpty: false,
			Default:   "default",
		}

		g.Expect(st.Skip()).Should(BeTrue(),
			"Skip method should return false if the key of the struct tag is an empty string!")
	})

	t.Run("Skip with - key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		st := &StructTag{
			Key:       "-",
			OmitEmpty: false,
			Default:   "default",
		}

		g.Expect(st.Skip()).Should(BeTrue(),
			"Skip method should return false if the key of the struct tag is a `-` string!")
	})

	t.Run("Skip with valid key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		st := &StructTag{
			Key:       "valid.key",
			OmitEmpty: false,
			Default:   "default",
		}

		g.Expect(st.Skip()).Should(BeFalse(),
			"Skip method should return true if the key of the struct tag is a valid string!")
	})
}

func TestStructTagFlag(t *testing.T) {
	t.Run("IsValid with empty key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		stf := &structTagFlag{
			Key:   "",
			Value: "",
		}

		g.Expect(stf.IsValid()).Should(BeFalse(),
			"IsValid method should return false if the key of the struct tag flag is an empty string!")
	})

	t.Run("IsValid with non-empty key", func(t *testing.T) {
		g := NewGomegaWithT(t)

		stf := &structTagFlag{
			Key: "omitempty",
		}

		g.Expect(stf.IsValid()).Should(BeTrue(),
			"IsValid method should return true if the key of the struct tag flag is a non-empty string!")
	})
}

func TestParseStructTagFlag(t *testing.T) {
	t.Run("Empty string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := parseStructTagFlag("")

		g.Expect(err).Should(HaveOccurred(),
			"Parsing an empty string as struct tag flag should yield an error!")
	})

	t.Run("Bool flag", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &structTagFlag{
			Key: "omitempty",
		}

		stf, err := parseStructTagFlag("omitempty")

		g.Expect(err).Should(BeNil(),
			"Parsing a non-empty struc tag flag string should not return an error!")

		g.Expect(stf).Should(Equal(expected), "Mismatch in expected and returned StructTagFlag!")
	})

	t.Run("Key/value flag", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &structTagFlag{
			Key:   "default",
			Value: "default value",
		}

		stf, err := parseStructTagFlag("default=default value")

		g.Expect(err).Should(BeNil(),
			"Parsing a non-empty struc tag flag string should not return an error!")

		g.Expect(stf).Should(Equal(expected), "Mismatch in expected and returned StructTagFlag!")
	})
}

func TestParseStructTag(t *testing.T) {
	t.Run("Empty string", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := ParseStructTag("")

		g.Expect(err).Should(HaveOccurred(),
			"Parsing an empty string should yield an error!")
	})

	t.Run("Key with no flags", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &StructTag{
			Key: "testTag",
		}

		st, err := ParseStructTag("testTag")

		g.Expect(err).Should(BeNil(),
			"Parsing a non-empty structag string should not return an error!")

		g.Expect(st).Should(Equal(expected), "Mismatch in expected and returned StructTag!")
	})

	t.Run("Key with supported flags", func(t *testing.T) {
		g := NewGomegaWithT(t)

		expected := &StructTag{
			Key:       "testTag",
			OmitEmpty: true,
			Default:   "test",
		}

		st, err := ParseStructTag("testTag,omitempty,default=test")

		g.Expect(err).Should(BeNil(),
			"Parsing a valid structag string should not return an error!")

		g.Expect(st).Should(Equal(expected), "Mismatch in expected and returned StructTag!")
	})

	t.Run("Key with unsupported flags", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := ParseStructTag("testTag,omitempty,invalidFlag")

		g.Expect(err).Should(HaveOccurred(),
			"Parsing a structag string with invalid flags whould returtn an error!")
	})

	t.Run("Key with empty flags", func(t *testing.T) {
		g := NewGomegaWithT(t)

		_, err := ParseStructTag("testTag,,,,")

		g.Expect(err).Should(HaveOccurred(),
			"Parsing a structag string with valid key and empty flags should return an error!")
	})
}
