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
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const (
	// String containing delimiter characters valid in Java core
	Separators = "=: "
	// Character used for escaping separators
	EscapeChar = '\\'

	StructTagKey = "properties"
)

// UnEscapeSeparators replaces escaped Separators with their unescaped version in the given string.
func UnEscapeSeparators(s string) string {
	// Get the length of the s string
	length := len(s)

	// Return immediately if s string is empty
	if length == 0 {
		return s
	}

	// Convert s string to slice of rune
	orig := []rune(s)
	// Convert Separators to slice of rune
	sep := []rune(Separators)
	// Create new slice holding the escaped string
	newSlice := make([]rune, 0)

	// Index from where we need to copy the data to the new slice
	startIdx := 0
	// Track previous index
	prevIdx := 0

	// Iterate over the original string to find separator characters
	for idx, c := range orig {
		// Set previous index by making sure that it's value is inbound
		if idx > 0 {
			prevIdx = idx - 1
		}
		// Iterate over the separator characters.
		for _, sp := range sep {
			// If there is a separator match and the previous is an escape character
			// then copy data up to the previous index to the new string then append
			// the current leaving out the previous character.
			if c == sp && orig[prevIdx] == EscapeChar {
				newSlice = append(newSlice, orig[startIdx:prevIdx]...)
				newSlice = append(newSlice, c)
				startIdx = idx + 1
				break
			}
		}
	}
	// Make sure that all the original string is copied to the new
	if startIdx <= (length - 1) {
		newSlice = append(newSlice, orig[startIdx:]...)
	}

	return string(newSlice)
}

// EscapeSeparators returns the given s string having the Separators escaped.
func EscapeSeparators(s string) string {
	// Get the length of the s string
	length := len(s)

	// Return immediately if s string is empty
	if length == 0 {
		return s
	}

	// Convert s string to slice of rune
	orig := []rune(s)
	// Convert Separators to slice of rune
	sep := []rune(Separators)
	// Create new slice holding the escaped string
	newSlice := make([]rune, 0)

	// Index from where we need to copy the data to the new slice
	startIdx := 0
	// Track previous index
	prevIdx := 0

	// Iterate over the original string to find separator characters
	for idx, c := range orig {
		// Set previous index by making sure that it's value is inbound
		if idx > 0 {
			prevIdx = idx - 1
		}
		// Iterate over the separator characters
		for _, sp := range sep {
			// If there is a separator match and the previous is not an escape character
			// then copy data up to the current index to the new string then append the
			// escape and the current characters.
			if c == sp && orig[prevIdx] != EscapeChar {
				newSlice = append(newSlice, orig[startIdx:idx]...)
				newSlice = append(newSlice, []rune{EscapeChar, c}...)
				startIdx = idx + 1
				break
			}
		}
	}
	// Make sure that all the original string is copied to the new
	if startIdx <= (length - 1) {
		newSlice = append(newSlice, orig[startIdx:]...)
	}

	return string(newSlice)
}

// GetSeparator return the separator character and its index if it is fond in the given string.
// Otherwise a NoSeparatorDetectedError is returned.
func GetSeparator(s string) (string, int, error) {
	// Index of the detected separator.
	var sepIdx int
	// Detected separator.
	var sep string

	// Get the length of the s string
	length := len(s)

	// Return immediately if s string is empty
	if length == 0 {
		return sep, sepIdx, &NoSeparatorFoundError{s}
	}

	// Convert s string to slice of rune
	r := []rune(s)
	// Convert Separators to slice of rune
	separators := []rune(Separators)

	// Track previous index
	prevIdx := 0

	// Iterate over the input string
	for idx, c := range r {
		// Avoid out of bound access
		if idx > 0 {
			prevIdx = idx - 1
		}
		// Iterate ofer the list of separators
		for _, sp := range separators {
			// If the current character is a separator and it is not escaped
			// than separator is found.
			if c == sp && r[prevIdx] != EscapeChar {
				sep = string(c)
				sepIdx = idx
				break
			}
		}
		// Stop iteration if separator is already found
		if sep != "" {
			break
		}
	}

	// Return with error if no separator is found.
	if sep == "" && sepIdx == 0 {
		return sep, sepIdx, &NoSeparatorFoundError{s}
	}

	return sep, sepIdx, nil
}

type Loader struct {
	// Data read from Scanner
	lines []string
}

func (l *Loader) Load(r io.Reader) (*Properties, error) {
	l.load(r)

	if len(l.lines) == 0 {
		return &Properties{}, fmt.Errorf("no data was loaded")
	}

	return l.parse()
}

func (l *Loader) load(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		l.lines = append(l.lines, line)
	}
}

func (l *Loader) parse() (*Properties, error) {
	// New Properties object.
	newProperties := NewProperties()

	// Temporary store for holding chunks of multiline property
	var property strings.Builder
	var comment strings.Builder

	for _, line := range l.lines {
		// Ignore empty lines
		if line == "" {
			property.Reset()
			comment.Reset()
			continue
		}

		// Comment lines start with either # or ! characters.
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			if _, err := comment.WriteString(line + "\n"); err != nil {
				return nil, err
			}
			continue
		}

		// Check for multiline property by looking for \ as the last character escaping the new line.
		if strings.HasSuffix(line, "\\") {
			if _, err := property.WriteString(strings.TrimSuffix(line, "\\")); err != nil {
				return nil, err
			}
			continue
		}

		property.WriteString(line)

		// Parse property from the string
		p, err := getPropertyFromString(property.String(), comment.String())
		if err != nil {
			return nil, err
		}

		// Reset property string
		property.Reset()
		comment.Reset()

		// Add Property to Properties object
		newProperties.Put(p)
	}
	return newProperties, nil
}

func NewLoader() *Loader {
	return &Loader{}
}

// NewFromString returns a Properties object containing the data gathered
// by parsing the s string.
func NewFromString(s string) (*Properties, error) {
	l := NewLoader()
	return l.Load(strings.NewReader(s))
}

// NewFromFile returns a Properties object containing the data gathered
// by parsing the file on the given filesystem path.
func NewFromFile(path string) (*Properties, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return &Properties{}, fmt.Errorf("cannot open file at %v", path)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	l := NewLoader()
	return l.Load(file)
}

type NoSeparatorFoundError struct {
	Property string
}

func (e *NoSeparatorFoundError) Error() string {
	return fmt.Sprintf("no separator detected for property: %s", e.Property)
}
