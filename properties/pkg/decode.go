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
	"reflect"

	"github.com/banzaicloud/kafka-operator/properties/internal/utils"
)

// Unmarshaler is the interface implemented by types that can unmarshal Properties
// by themselves.
type Unmarshaler interface {
	UnmarshalProperties(p *Properties) error
}

// Unmarshal updates the values of object pointed by v using the data read from p Properties.
// If v is nil or not a pointer the Unmarshal returns an InvalidUnmarshalError.
//
// If v implements the Unmarshaler interface, Unmarshal calls the UnmarshalProperties method.
func Unmarshal(p *Properties, v interface{}) error {
	unmarshalerType := reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	vValue := reflect.ValueOf(v)

	if !vValue.IsValid() {
		return &InvalidUnmarshalError{}
	}

	var vType reflect.Type

	if vValue.Kind() == reflect.Ptr {
		if vValue.IsNil() {
			return &InvalidUnmarshalError{vValue.Type()}
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
	if vValue.Kind() != reflect.Ptr || vValue.IsNil() {
		return &InvalidUnmarshalError{vValue.Type()}
	}

	vType = vValue.Elem().Type()

	// The v interface is expected to be a pointer to a struct.
	if vType.Kind() != reflect.Struct {
		return &InvalidUnmarshalError{vType}
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
			return &UnmarshalImmutableFieldError{vTypeField}
		}

		// Match type of field with the type of property Property from p Properties
		var propType PropertyType
		switch vFieldValue.Kind() {
		default:
			return &UnmarshalFieldTypeError{vTypeField, vFieldValue.Type(), nil}
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
				return &UnmarshalFieldTypeError{vTypeField, vFieldValue.Type(), vFieldValue.Type().Elem()}
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

// An InvalidUnmarshalError describes an invalid v argument passed to Unmarshal.
// The v argument to Unmarshal must be a non-nil pointer which points to a struct.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "properties: cannot unmarshal (nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return fmt.Sprintf("properties: cannot unmarshal (non-pointer %s)", e.Type)
	}

	return fmt.Sprintf("properties: cannot unmarshal (%s)", e.Type)
}

// An UnmarshalFieldTypeError describes an exported StructField and its Type
// which value triggered an error due to its unsupported type.
type UnmarshalFieldTypeError struct {
	Field    reflect.StructField
	Type     reflect.Type
	ElemType reflect.Type
}

func (e UnmarshalFieldTypeError) Error() string {
	if e.Type.Kind() == reflect.Slice && e.ElemType != nil {
		return fmt.Sprintf("properties: cannot unmarshal %q field with type []%s", e.Field.Name, e.ElemType)
	}
	return fmt.Sprintf("properties: cannot unmarshal %q field with type %q", e.Field.Name, e.Type)
}

// An UnmarshalImmutableFieldError describes an exported StructField
// which value cannot be updated as it is immutable.
type UnmarshalImmutableFieldError struct {
	Field reflect.StructField
}

func (e UnmarshalImmutableFieldError) Error() string {
	return fmt.Sprintf("properties: cannot unmarshal immutable field (%s)", e.Field.Name)
}
