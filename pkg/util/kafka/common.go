// Copyright © 2019 Banzai Cloud
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

package kafka

import (
	"fmt"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/util"
)

const (
	// AllBrokerServiceTemplate template for Kafka headless service
	AllBrokerServiceTemplate = "%s-all-broker"
	// HeadlessServiceTemplate template for Kafka headless service
	HeadlessServiceTemplate = "%s-headless"
)

var commonAclString = "User:%s,Topic,LITERAL,%s,Describe,Allow,*"
var createAclString = "User:%s,Topic,LITERAL,%s,Create,Allow,*"
var writeAclString = "User:%s,Topic,LITERAL,%s,Write,Allow,*"
var readAclString = "User:%s,Topic,LITERAL,%s,Read,Allow,*"
var readGroupAclString = "User:%s,Group,LITERAL,*,Read,Allow,*"

func GrantsToACLStrings(dn string, grants []v1alpha1.UserTopicGrant) []string {
	acls := make([]string, 0)
	for _, x := range grants {
		cmn := fmt.Sprintf(commonAclString, dn, x.TopicName)
		if !util.StringSliceContains(acls, cmn) {
			acls = append(acls, cmn)
		}
		switch x.AccessType {
		case v1alpha1.KafkaAccessTypeRead:
			readAcl := fmt.Sprintf(readAclString, dn, x.TopicName)
			readGroupAcl := fmt.Sprintf(readGroupAclString, dn)
			for _, y := range []string{readAcl, readGroupAcl} {
				if !util.StringSliceContains(acls, y) {
					acls = append(acls, y)
				}
			}
		case v1alpha1.KafkaAccessTypeWrite:
			createAcl := fmt.Sprintf(createAclString, dn, x.TopicName)
			writeAcl := fmt.Sprintf(writeAclString, dn, x.TopicName)
			for _, y := range []string{createAcl, writeAcl} {
				if !util.StringSliceContains(acls, y) {
					acls = append(acls, y)
				}
			}
		}
	}
	return acls
}
