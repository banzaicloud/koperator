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

	"github.com/pkg/errors"
)

const (
	StructTagDelimiter = ","
	StructTagSeparator = "="
)

type StructTag struct {
	// Key name
	Key string
	// Flags
	OmitEmpty bool
	Default   string
}

func (t StructTag) Skip() bool {
	if t.Key == "" || t.Key == "-" {
		return true
	}
	return false
}

type StructTagFlag struct {
	Key   string
	Value string
}

func (f StructTagFlag) IsValid() bool {
	if f.Key != "" {
		return true
	}
	return false
}

func parseStructTagFlag(s string) (*StructTagFlag, error) {
	if s == "" {
		return nil, errors.Errorf("struct tag flag must not be empty string")
	}
	// Create empty flag
	flag := StructTagFlag{}
	// Split s by separator to the key and the value of the struct tag flag
	f := strings.SplitN(s, StructTagSeparator, 2)
	// Set key as it must be always present at this point
	flag.Key = f[0]
	// There are flags with values like the "default" flag in the following example:
	//     "properties:test,omitempty,default=true"
	if len(f) == 2 {
		flag.Value = f[1]
	}
	return &flag, nil
}

// ParseStructTag returns a map of struct tags by parsing s.
func ParseStructTag(s string) (*StructTag, error) {
	// Empty struct tag string is treated as an error
	if s == "" {
		return nil, errors.Errorf("struct tag must not be empty string")
	}
	// Create empty struct tag
	st := &StructTag{}
	// Split struct tag by delimiter character
	items := strings.Split(s, StructTagDelimiter)
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
				return nil, errors.Errorf("struct tag flag `%v` is not supported", f)
			}
		}
	}
	return st, nil
}
