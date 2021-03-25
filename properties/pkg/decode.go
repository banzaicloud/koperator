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

// Unmarshaler is the interface implemented by types that can unmarshal Properties
// by themselves.
type Unmarshaler interface {
	UnmarshalProperties(p *Properties) error
}

// Unmarshal updates the values of object pointed by v using the data read from p Properties.
// If v is nil or not a pointer the Unmarshal returns an error.
//
// If v implements the Unmarshaler interface, Unmarshal calls the UnmarshalProperties method.
func Unmarshal(p *Properties, v interface{}) error {
	unmarshalerType := reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	vValue := reflect.ValueOf(v)

	if !vValue.IsValid() {
		return errors.New("properties: cannot unmarshal (nil)")
	}

	var vType reflect.Type

	if vValue.Kind() == reflect.Ptr {
		if vValue.IsNil() {
			return errors.New("properties: cannot unmarshal (nil-pointer)")
		}
		vType = vValue.Elem().Type()
	} else {
		vType = vValue.Type()
	}

	if reflect.PtrTo(vType).Implements(unmarshalerType) {
		if u, ok := vValue.Interface().(Unmarshaler); ok {
			return u.UnmarshalProperties(p)
		}
	}
	return unmarshal(p, v)
}

// unmarshal reads exported fields of v and updates their values
// with corresponding data from p Properties.
func unmarshal(p *Properties, v interface{}) error {
	var vValue reflect.Value
	var vType reflect.Type

	vValue = reflect.ValueOf(v)

	// The goal is to perform in-place update therefore v interface needs to be a non-nil pointer.
	if vValue.Kind() != reflect.Ptr {
		return errors.New("properties: cannot unmarshal into non-pointer object")
	} else if vValue.IsNil() {
		return errors.New("properties: cannot unmarshal into nil-pointer object")
	}

	vType = vValue.Elem().Type()

	// The v interface is expected to be a pointer to a struct.
	if vType.Kind() != reflect.Struct {
		return errors.New("properties: cannot unmarshal into non-struct object")
	}

	// Iterate over the fields of the struct and set its values from p Properties
	for i, numFields := 0, vType.NumField(); i < numFields; i++ {
		vTypeField := vType.Field(i)

		// Check if StructTagKey key is present for this struct field and move on to the next field if not.
		tag, found := vTypeField.Tag.Lookup(StructTagKey)
		if !found || tag == "" {
			continue
		}

		// Parse tag string to StructTag object
		st, err := utils.ParseStructTag(tag)
		if err != nil {
			return err
		}
		// Move on to the next field if the field needs to be skipped
		// either because the tag key is empty or its value is `-`.
		if st.Skip() {
			continue
		}
		// Move on to the next field if the tag key is not present in p Properties
		property, found := p.Get(st.Key)
		if !found {
			continue
		}

		// Check if the field value can be set and an return error otherwise as the desired behaviour is to update
		// it's value if it is present in p Properties.
		vFieldValue := vValue.Elem().Field(i)
		if !vFieldValue.CanSet() {
			return errors.NewWithDetails("properties: cannot unmarshal immutable field", "field", vTypeField.Name)
		}

		// Match type of field with the type of property Property from p Properties
		var propType PropertyType
		//nolint:exhaustive
		switch vFieldValue.Kind() {
		default:
			return errors.NewWithDetails("properties: cannot unmarshal field with unsupported type", "field", vTypeField.Name, "type", vFieldValue.Type().Name())
		case reflect.String:
			propType = String
		case reflect.Int64:
			propType = Int
		case reflect.Float64:
			propType = Float
		case reflect.Bool:
			propType = Bool
		case reflect.Slice:
			vFieldValueElemKind := vFieldValue.Type().Elem().Kind()
			// Only []string is supported.
			if vFieldValueElemKind != reflect.String {
				return errors.NewWithDetails("properties: cannot unmarshal field unsupported slice type", "field", vTypeField.Name, "type", vFieldValue.Type().Elem().Name())
			}
			propType = List
		}

		// Get property value with type matching with the field type
		propValue, err := property.GetByType(propType)
		if err != nil {
			return err
		}

		// Get a new Value for property value
		newValue := reflect.ValueOf(propValue)

		// Update the field with the property value
		vFieldValue.Set(newValue)
	}
	return nil
}
