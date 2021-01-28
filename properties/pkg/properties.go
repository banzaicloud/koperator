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
	"strings"
	"sync"
)

const (
	// Default delimiter used for exporting Property
	DefaultSeparator = "="
)

type MergeOption uint8

const (
	AllowOverwrite MergeOption = iota
	OnlyDefaults
)

type keyIndex struct {
	key   string
	index uint
}

type keyIndexList []keyIndex

func (k keyIndexList) Len() int {
	return len(k)
}

func (k keyIndexList) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

func (k keyIndexList) Less(i, j int) bool {
	return k[i].index < k[j].index
}

// Properties is used to store a group of Property items belong together.
// It also supports number of operations on the stored Property items.
type Properties struct {
	// Map of Property objects
	properties map[string]Property
	// Map of Property keys and indices for bookkeeping  their original
	// order in the Properties document.
	keys map[string]keyIndex
	// Index for the next Property to be added to Properties
	nextKeyIndex uint
	// State lock to prevent concurrent updates of Properties
	mutex sync.RWMutex
}

// Get a Property using by its name.
func (p *Properties) Get(key string) (Property, bool) {
	// Acquire read lock
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Get property if exists
	propByKey, found := p.properties[key]
	if !found {
		return Property{}, false
	}
	return propByKey, true
}

// Get list of Property names.
func (p *Properties) Keys() []string {
	// Acquire read lock
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Create an keyIndexList with the size of the keys map
	keyIdxList := make(keyIndexList, 0, len(p.keys))

	// Add keyIndex items from keys map to keyIndexList before sorting
	for _, keyIdx := range p.keys {
		keyIdxList = append(keyIdxList, keyIdx)
	}

	// Sort keys in keyIndexList by their index
	sort.Sort(keyIdxList)

	// Create a string slice with the size of the keys map
	// holding the properties keys in order
	keys := make([]string, 0, len(p.keys))

	// Retrieve the keys from the sorted keyIndexList
	for _, keyIdx := range keyIdxList {
		keys = append(keys, keyIdx.key)
	}

	return keys
}

// Get number of items in Properties
func (p *Properties) Len() int {
	// Acquire read lock
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.keys)
}

// Add a new Property to Properties with the given name and value.
func (p *Properties) Set(key string, value interface{}, comment string) error {
	prop := Property{}
	err := prop.set(key, value, comment)
	if err != nil {
		return err
	}

	p.Put(prop)
	return nil
}

// Put adds the prop Property to Properties
func (p *Properties) Put(property Property) {
	// Acquire RW lock before updating internal state
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.put(property)
}

func (p *Properties) put(property Property) {
	if _, found := p.properties[property.key]; !found {
		p.keys[property.key] = keyIndex{key: property.key, index: p.nextKeyIndex}
		p.nextKeyIndex++
	}
	p.properties[property.key] = property
}

// Delete the Property with the given name.
func (p *Properties) Delete(key string) {
	// Acquire RW lock before updating internal state
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Delete Property from key index
	delete(p.keys, key)

	// Delete Property from map
	delete(p.properties, key)
}

// Merge two Properties objects by updating Property values in p from m.
func (p *Properties) Merge(m *Properties) {
	p.merge(m, AllowOverwrite)
}

// Merge two Properties objects by updating Property values in p from m if format has a default value
func (p *Properties) MergeDefaults(m *Properties) {
	p.merge(m, OnlyDefaults)
}

func (p *Properties) merge(m *Properties, option MergeOption) {
	// Check it t is a nil-pointer and if so just return
	if m == nil {
		return
	}
	// Acquire read lock for m before iterating over m
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Acquire RW lock before updating internal state
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Merge m to p
	for _, mProp := range m.properties {
		switch option {
		case OnlyDefaults:
			pProp, found := p.properties[mProp.key]
			if found && !pProp.IsEmpty() {
				continue
			}
			fallthrough
		case AllowOverwrite:
			fallthrough
		default:
			p.put(mProp)
		}
	}
}

// Equal compares two Properties which are equal if they have the same
// set of Property objects.
// The order of the keys is not taken into consideration.
func (p *Properties) Equal(t *Properties) bool {
	// Check it t is a nil-pointer and if so then return false
	if t == nil {
		return false
	}
	// Acquire read lock for t before iterating over t
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Acquire read lock for p before iterating over p
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Two Properties object cannot be equal if their length is not equal.
	if len(p.keys) != len(t.keys) {
		return false
	}

	// Check if all keys from t Properties is present in p Properties and
	// the corresponding Property objects are also equal. If not then the
	// Properties objects are not equal.
	for tKey := range t.keys {
		pProp, found := p.properties[tKey]
		if !found {
			return false
		}

		tProp, found := t.properties[tKey]
		// This should not happen
		if !found {
			return false
		}

		if !pProp.Equal(tProp) {
			return false
		}
	}
	return true
}

// String returns string representation of Properties.
func (p *Properties) String() string {
	var props strings.Builder

	// Acquire read lock for t before iterating over t
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, key := range p.Keys() {
		if prop, found := p.Get(key); found {
			props.WriteString(fmt.Sprintf("%s\n", prop))
		}
	}
	return props.String()
}

// Provides a custom JSON Marshal interface
func (p *Properties) MarshalJSON() ([]byte, error) {
	// Acquire read lock for t before iterating over t
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	props := make(map[string]string)

	for key, prop := range p.properties {
		props[key] = prop.Value()
	}

	return json.Marshal(&struct {
		Properties map[string]string `json:"properties"`
	}{
		Properties: props,
	},
	)
}

// NewProperties returns a new and empty Properties object.
func NewProperties() *Properties {
	return &Properties{
		properties: make(map[string]Property, 0),
		keys:       make(map[string]keyIndex, 0),
	}
}
