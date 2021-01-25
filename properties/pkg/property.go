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
	"strconv"
	"strings"

	"emperror.dev/errors"
)

const (
	ListSeparator = ","
)

type PropertyType uint

const (
	Int PropertyType = iota
	Float
	Bool
	String
	List
	Invalid
)

// Property is used to store the key and value for a property. It also provides methods
// to cast the value of the property into different types.
type Property struct {
	// key holds the name of the property in string.
	key string
	// value stores the value of the property in string.
	value string
	// comment stores comments for the property
	comment string
}

// Key method return the name of the Property.
func (p *Property) Key() string {
	return p.key
}

// Value returns the value of the Property.
func (p *Property) Value() string {
	return p.value
}

// Comment returns the comment for the Property.
func (p *Property) Comment() string {
	return p.comment
}

func (p *Property) Equal(t Property) bool {
	if p.key != t.key || p.value != t.value {
		return false
	}
	return true
}

// String implements the Stringer interface.
func (p Property) String() string {
	return fmt.Sprintf("%s%s%s", EscapeSeparators(p.key), DefaultSeparator, p.value)
}

// Int converts the Property value to Int64.
func (p Property) Int() (int64, error) {
	return strconv.ParseInt(p.value, 10, 64)
}

// Float converts the Property value to Float64.
func (p Property) Float() (float64, error) {
	return strconv.ParseFloat(p.value, 64)
}

// Bool converts the Property value to Boolean.
func (p Property) Bool() (bool, error) {
	return strconv.ParseBool(p.value)
}

// List converts the value of the Property to a list of strings using the given separator.
func (p Property) List() ([]string, error) {
	return strings.Split(p.value, ListSeparator), nil
}

func (p Property) GetByType(t PropertyType) (interface{}, error) {
	switch t {
	case Int:
		return p.Int()
	case Float:
		return p.Float()
	case String:
		return p.Value(), nil
	case Bool:
		return p.Bool()
	case List:
		return p.List()
	default:
		return nil, errors.NewWithDetails("properties: unsupported type", "type", t)
	}
}

func (p *Property) set(k string, v interface{}, c string) error {
	p.key = k
	p.comment = c

	vValue := reflect.ValueOf(v)

	if !vValue.IsValid() {
		return errors.NewWithDetails("properties: invalid property", "property", k)
	}

	switch vValue.Kind() {
	case reflect.String:
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fallthrough
	case reflect.Float32, reflect.Float64:
		fallthrough
	case reflect.Bool:
		p.value = fmt.Sprintf("%v", v)
	case reflect.Slice:
		sliceKind := vValue.Type().Elem().Kind()

		switch sliceKind {
		case reflect.String:
			slice, ok := vValue.Interface().([]string)
			if !ok {
				return errors.NewWithDetails("properties: cannot load property into string slice", "property", k)
			}
			p.value = strings.Join(slice, ListSeparator)

		default:
			return errors.NewWithDetails("properties: unsupported slice type", "property", k, "type", sliceKind)
		}

	default:
		return errors.NewWithDetails("properties: unsupported type", "property", k)
	}

	return nil
}
