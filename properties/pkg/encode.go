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

	"emperror.dev/errors"
	"github.com/banzaicloud/kafka-operator/properties/internal/utils"
)

// Marshaler is the interface implemented by types that can marshal to Properties
// by themselves.
type Marshaler interface {
	MarshalProperties() (*Properties, error)
}

// Marshal returns Properties.
// If v is nil or not a pointer the Marshal returns an error.
//
// If v implements the Marshaler interface, Marshal calls the MarshalProperties method.
func Marshal(v interface{}) (*Properties, error) {
	marshalerType := reflect.TypeOf((*Marshaler)(nil)).Elem()
	vValue := reflect.ValueOf(v)

	if !vValue.IsValid() {
		return nil, errors.New("properties: cannot marshal (nil)")
	}

	var vType reflect.Type

	if vValue.Kind() == reflect.Ptr {
		if vValue.IsNil() {
			return nil, errors.New("properties: cannot marshal nil-pointer object")
		}
		vType = vValue.Elem().Type()
	} else {
		vType = vValue.Type()
	}

	if vType.Implements(marshalerType) {
		if u, ok := vValue.Interface().(Marshaler); ok {
			return u.MarshalProperties()
		}
	}
	return marshal(v)
}

func marshal(v interface{}) (*Properties, error) {
	vValue := reflect.Indirect(reflect.ValueOf(v))
	vType := vValue.Type()

	// The v interface is expected to be a pointer to a struct.
	if vValue.IsValid() && vType.Kind() != reflect.Struct {
		return nil, errors.New("properties: cannot marshal non-struct object")
	}

	properties := NewProperties()

	// Iterate over the fields of the struct and update config-tools with their value
	for i, numFields := 0, vType.NumField(); i < numFields; i++ {
		vTypeField := vType.Field(i)

		// Check if StructTagKey key is present for this struct field and move on to the next field if not.
		tag, found := vTypeField.Tag.Lookup(StructTagKey)
		if !found || tag == "" {
			continue
		}

		// Parse struct tag and move on to the next field if the tag name is empty.
		st, err := utils.ParseStructTag(tag)
		if err != nil {
			return nil, errors.WithDetails(err, "struct-tag", tag)
		}
		if st.Key == "" || st.Skip() {
			continue
		}

		// Get value for the struct field
		vFieldValue := vValue.Field(i)

		// Check if the value is the default for its type and skip the field
		// if `omitempty` flag is set.
		if vFieldValue.IsZero() && st.OmitEmpty {
			continue
		}
		err = properties.Set(st.Key, vFieldValue.Interface(), "")
		if err != nil {
			return nil, errors.WithDetails(err, "key", st.Key, "value", vFieldValue.Interface())
		}
	}
	return properties, nil
}
