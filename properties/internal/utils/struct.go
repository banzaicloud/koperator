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
	"strings"

	"emperror.dev/errors"
)

const (
	// Delimiter used to separate items in struct tags.
	structTagDelimiter = ","
	// Separator used for defining key/value flags in struct tags.
	structTagSeparator = "="
)

// StructTag stores information about the struct tags which includes the name of the Key and optional flags like
// `omitempty` and `default`.
//
// The following string can be parsed as StructTag:
//  s := "properties:test_key,omitempty,default=test_key_default_value"
//  st, err := ParseStructTag(s)
// And information can be accessed:
//  st.Key       // returns "test_key"
//  st.OmitEmpty // return true
//  st.Default   // returns "test_key_default_value"
type StructTag struct {
	// Key name which is the first item of a struct tag
	Key string
	// Flags
	OmitEmpty bool
	Default   string
}

// Skip method returns true if the Key field of the StructTag is either an empty string or `-` meaning
// that the struct field should not be considered during marshal/unmarshal.
//
// Calling Skip method for st1 or st2 StructTag object will result false:
//  s1 := "properties:,omitempty,default=test_key_default_value"
//  st1, err := ParseStructTag(s1)
//  st1.Skip()
// Or
//  s2 := "properties:-,omitempty,default=test_key_default_value"
//  st2, err := ParseStructTag(s2)
//  st2.Skip()
func (t StructTag) Skip() bool {
	if t.Key == "" || t.Key == "-" {
		return true
	}
	return false
}

// structTagFlag holds information about flags defined for a struct tag in a Key/Value format.
type structTagFlag struct {
	Key   string
	Value string
}

// IsValid considers a f StructTagFlag valid if Key field is a non-empty string.
func (f structTagFlag) IsValid() bool {
	if f.Key != "" {
		return true
	}
	return false
}

// parseStructTagFlag returns a pointer to a StructTagFlag object holding the information resulted from parsing the
// s string as a struct tag flag.
func parseStructTagFlag(s string) (*structTagFlag, error) {
	if s == "" {
		return nil, errors.New("struct tag flag must not be empty string")
	}
	// Create empty flag
	flag := structTagFlag{}
	// Split s by separator to the key and the value of the struct tag flag
	f := strings.SplitN(s, structTagSeparator, 2)
	// Set key as it must be always present at this point
	flag.Key = f[0]
	// There are flags with values like the "default" flag in the following example:
	//     "properties:test,omitempty,default=true"
	if len(f) == 2 {
		flag.Value = f[1]
	}
	return &flag, nil
}

// ParseStructTag returns a pointer to a StructTag object holding the information resulted from parsing the
// s string as a struct tag.
func ParseStructTag(s string) (*StructTag, error) {
	// Empty struct tag string is treated as an error
	if s == "" {
		return nil, errors.New("struct tag must not be empty string")
	}
	// Create empty struct tag
	st := &StructTag{}
	// Split struct tag by delimiter character
	items := strings.Split(s, structTagDelimiter)
	st.Key = items[0]
	// Parse flags if they are present in the struct tag
	if len(items) > 1 {
		// Iterate over the flags
		for _, f := range items[1:] {
			// Parse struct tag flag
			flag, err := parseStructTagFlag(f)
			if err != nil {
				return nil, err
			}
			// Handle supported flags
			switch flag.Key {
			case "omitempty":
				st.OmitEmpty = true
			case "default":
				st.Default = flag.Value
			default:
				return nil, errors.NewWithDetails("struct tag flag is not supported", "flag", f)
			}
		}
	}
	return st, nil
}
