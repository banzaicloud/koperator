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

package kafkaclient

import (
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
)

// CreateUserACLs creates Kafka ACLs for the given access type and user
func (k *kafkaClient) CreateUserACLs(accessType v1alpha1.KafkaAccessType, dn string, topic string) (err error) {
	userName := fmt.Sprintf("User:%s", dn)
	switch accessType {
	case v1alpha1.KafkaAccessTypeRead:
		return k.createReadACLs(userName, topic)
	case v1alpha1.KafkaAccessTypeWrite:
		return k.createWriteACLs(userName, topic)
	default:
		return errorfactory.New(errorfactory.InternalError{}, fmt.Errorf("unknown type: %s", accessType), "unrecognized access type")
	}
}

// DeleteUserACLs removes all ACLs for a given user
func (k *kafkaClient) DeleteUserACLs(dn string) (err error) {
	matches, err := k.admin.DeleteACL(sarama.AclFilter{
		Principal: &dn,
	}, false)
	if err != nil {
		return
	}
	for _, x := range matches {
		if x.Err != sarama.ErrNoError {
			return x.Err
		}
	}
	return
}

func (k *kafkaClient) createReadACLs(dn string, topic string) (err error) {
	if err = k.createCommonACLs(dn, topic); err != nil {
		return
	}

	// READ on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: sarama.AclPatternLiteral,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationRead,
		PermissionType: sarama.AclPermissionAllow,
	}); err != nil {
		return
	}

	// READ on groups
	err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceGroup,
		ResourceName:        "*",
		ResourcePatternType: sarama.AclPatternLiteral,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationRead,
		PermissionType: sarama.AclPermissionAllow,
	})

	return
}

func (k *kafkaClient) createWriteACLs(dn string, topic string) (err error) {
	if err = k.createCommonACLs(dn, topic); err != nil {
		return
	}

	// WRITE on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: sarama.AclPatternLiteral,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationWrite,
		PermissionType: sarama.AclPermissionAllow,
	}); err != nil {
		return
	}

	// CREATE on topic
	err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: sarama.AclPatternLiteral,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationCreate,
		PermissionType: sarama.AclPermissionAllow,
	})

	return
}

func (k *kafkaClient) createCommonACLs(dn string, topic string) error {
	// DESCRIBE on topic
	return k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: sarama.AclPatternLiteral,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationDescribe,
		PermissionType: sarama.AclPermissionAllow,
	})
}
