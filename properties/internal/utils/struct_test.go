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

	"github.com/google/go-cmp/cmp"
)

func TestStructTag(t *testing.T) {
	t.Run("Skip with empty key", func(t *testing.T) {
		st := &StructTag{
			Key:       "",
			OmitEmpty: false,
			Default:   "default",
		}

		if !st.Skip() {
			t.Errorf("Skip method should return false if the key of the struct tag is an empty string!")
		}
	})

	t.Run("Skip with - key", func(t *testing.T) {
		st := &StructTag{
			Key:       "-",
			OmitEmpty: false,
			Default:   "default",
		}

		if !st.Skip() {
			t.Errorf("Skip method should return false if the key of the struct tag is a `-` string!")
		}
	})

	t.Run("Skip with valid key", func(t *testing.T) {
		st := &StructTag{
			Key:       "valid.key",
			OmitEmpty: false,
			Default:   "default",
		}

		if st.Skip() {
			t.Errorf("Skip method should return true if the key of the struct tag is a valid string!")
		}
	})
}

func TestStructTagFlag(t *testing.T) {
	t.Run("IsValid with empty key", func(t *testing.T) {
		stf := &structTagFlag{
			Key:   "",
			Value: "",
		}

		if stf.IsValid() {
			t.Errorf("IsValid method should return false if the key of the struct tag flag is an empty string!")
		}
	})

	t.Run("IsValid with non-empty key", func(t *testing.T) {
		stf := &structTagFlag{
			Key: "omitempty",
		}

		if !stf.IsValid() {
			t.Errorf("IsValid method should return true if the key of the struct tag flag is a non-empty string!")
		}
	})
}

func TestParseStructTagFlag(t *testing.T) {
	t.Run("Empty string", func(t *testing.T) {
		if _, err := parseStructTagFlag(""); err == nil {
			t.Errorf("Parsing an empty string as struct tag flag should yield an error!")
		}
	})

	t.Run("Bool flag", func(t *testing.T) {
		expected := &structTagFlag{
			Key: "omitempty",
		}
		stf, err := parseStructTagFlag("omitempty")

		if err != nil {
			t.Errorf("Parsing a non-empty struc tag flag string should not return an error!")
		}

		if !cmp.Equal(stf, expected) {
			t.Errorf("Mismatch in expected and returned StructTagFlag!\nExpected: %v\nGot: %v\n\n %v\n", expected, stf, cmp.Diff(expected, stf))
		}
	})

	t.Run("Key/value flag", func(t *testing.T) {
		expected := &structTagFlag{
			Key:   "default",
			Value: "default value",
		}
		stf, err := parseStructTagFlag("default=default value")

		if err != nil {
			t.Errorf("Parsing a non-empty struc tag flag string should not return an error!")
		}

		if !cmp.Equal(stf, expected) {
			t.Errorf("Mismatch in expected and returned StructTagFlag!\nExpected: %v\nGot: %v\n\n %v\n", expected, stf, cmp.Diff(expected, stf))
		}
	})
}

func TestParseStructTag(t *testing.T) {
	t.Run("Empty string", func(t *testing.T) {
		if _, err := ParseStructTag(""); err == nil {
			t.Errorf("Parsing an empty string should yield an error!")
		}
	})

	t.Run("Key with no flags", func(t *testing.T) {
		expected := &StructTag{
			Key: "testTag",
		}
		st, err := ParseStructTag("testTag")

		if err != nil {
			t.Errorf("Parsing a non-empty structag string should not return an error")
		}

		if !cmp.Equal(st, expected) {
			t.Errorf("Mismatch in expected and returned StructTag!\nExpected: %v\nGot: %v\n\n %v\n", expected, st, cmp.Diff(expected, st))
		}
	})

	t.Run("Key with supported flags", func(t *testing.T) {
		expected := &StructTag{
			Key:       "testTag",
			OmitEmpty: true,
			Default:   "test",
		}

		st, err := ParseStructTag("testTag,omitempty,default=test")

		if err != nil {
			t.Errorf("Parsing a valid structag string should not return an error")
		}

		if !cmp.Equal(st, expected) {
			t.Errorf("Mismatch in expected and returned StructTag!\nExpected: %v\nGot: %v\n\n %v\n", expected, st, cmp.Diff(expected, st))
		}
	})

	t.Run("Key with unsupported flags", func(t *testing.T) {

		_, err := ParseStructTag("testTag,omitempty,invalidFlag")

		if err == nil {
			t.Errorf("Parsing a structag string with invalid flags whould returtn an error!")
		}

	})

	t.Run("Key with empty flags", func(t *testing.T) {
		if _, err := ParseStructTag("testTag,,,,"); err == nil {
			t.Errorf("Parsing a structag string with valid key and empty flags should return an error")
		}
	})
}
