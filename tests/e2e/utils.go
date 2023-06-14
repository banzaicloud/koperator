// Copyright © 2023 Cisco Systems, Inc. and/or its affiliates
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

package e2e

import (
	"fmt"
	"sort"
	"strings"
)

const (
	kubectlNotFoundErrorMsg = "NotFound"
)

// Returns the union of the slices from argument.
func _stringSlicesUnion(sliceA, sliceB []string) []string {
	if len(sliceA) == 0 || len(sliceB) == 0 {
		return nil
	}

	sort.Slice(sliceA, func(i int, j int) bool {
		return sliceA[i] < sliceA[j]
	})
	sort.Slice(sliceB, func(i int, j int) bool {
		return sliceB[i] < sliceB[j]
	})

	var union []string
	for i := range sliceB {
		for j := range sliceA {
			if sliceB[i] < sliceA[j] {
				break
			} else if sliceB[i] == sliceA[j] {
				union = append(union, sliceB[i])
			}
		}
	}
	return union
}

func _kubectlArgExtender(args []string, logMsg, selector, names, namespace string, extraArgs []string) (string, []string) {
	if selector != "" {
		logMsg = fmt.Sprintf("%s selector: '%s'", logMsg, selector)
		args = append(args, fmt.Sprintf("--selector=%s", selector))
	} else if names != "" {
		logMsg = fmt.Sprintf("%s name(s): '%s'", logMsg, names)
		args = append(args, names)
	}
	if namespace != "" {
		logMsg = fmt.Sprintf("%s namespace: '%s'", logMsg, namespace)
	}
	if len(extraArgs) != 0 {
		logMsg = fmt.Sprintf("%s extraArgs: '%s'", logMsg, extraArgs)
		args = append(args, extraArgs...)
	}
	return logMsg, args
}

// _kubectlRemoveWarning removes those elements from the outputSlice parameter which contains kubectl warning message.
func _kubectlRemoveWarnings(outputSlice []string) []string {
	// Remove warning message pollution from the output
	result := make([]string, 0, len(outputSlice))
	for i := range outputSlice {
		if !strings.Contains(outputSlice[i], "Warning:") {
			result = append(result, outputSlice[i])
		}
	}
	return result
}

func _isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), kubectlNotFoundErrorMsg)
}
