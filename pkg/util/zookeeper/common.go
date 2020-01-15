// Copyright © 2020 Banzai Cloud
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

package zookeeper

import "strings"

// PrepareConnectionAddress prepares the proper address for Kafka and CC
// The required path for Kafka and CC looks 'example-1:2181,example-2:2181/kafka'
func PrepareConnectionAddress(zkAddresses []string, zkPath string) string {
	return strings.Join(zkAddresses, ",") + zkPath
}
