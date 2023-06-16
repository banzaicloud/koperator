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
	"sort"
)

// stringSlicesUnion returns the union of the slices from argument.
func stringSlicesUnion(sliceA, sliceB []string) []string {
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
