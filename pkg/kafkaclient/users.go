// Copyright © 2019 Cisco Systems, Inc. and/or its affiliates
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

	"github.com/IBM/sarama"
	"github.com/banzaicloud/koperator/api/v1alpha1"
	"github.com/banzaicloud/koperator/pkg/errorfactory"
)

// AclPatternTypeMapping maps patternType from v1alpha1.KafkaPatternType to sarama.AclResourcePatternType
func AclPatternTypeMapping(patternType v1alpha1.KafkaPatternType) sarama.AclResourcePatternType {
	switch patternType {
	case v1alpha1.KafkaPatternTypeAny:
		return sarama.AclPatternAny
	case v1alpha1.KafkaPatternTypeMatch:
		return sarama.AclPatternMatch
	case v1alpha1.KafkaPatternTypeLiteral:
		return sarama.AclPatternLiteral
	case v1alpha1.KafkaPatternTypePrefixed:
		return sarama.AclPatternPrefixed
	default:
		return sarama.AclPatternUnknown
	}
}

// CreateUserACLs creates Kafka ACLs for the given access type and user
// `literal` patternType will be used if patternType == ""
func (k *kafkaClient) CreateUserACLs(accessType v1alpha1.KafkaAccessType, patternType v1alpha1.KafkaPatternType, dn string, topic string) (err error) {
	userName := fmt.Sprintf("User:%s", dn)
	if patternType == "" {
		patternType = v1alpha1.KafkaPatternTypeDefault
	}
	aclPatternType := AclPatternTypeMapping(patternType)
	if aclPatternType == sarama.AclPatternUnknown {
		return errorfactory.New(errorfactory.InternalError{}, fmt.Errorf("unknown type: %s", patternType), "unrecognized pattern type")
	}
	switch accessType {
	case v1alpha1.KafkaAccessTypeRead:
		return k.createReadACLs(userName, topic, aclPatternType)
	case v1alpha1.KafkaAccessTypeWrite:
		return k.createWriteACLs(userName, topic, aclPatternType)
	default:
		return errorfactory.New(errorfactory.InternalError{}, fmt.Errorf("unknown type: %s", accessType), "unrecognized access type")
	}
}

func (k *kafkaClient) ListUserACLs() ([]sarama.ResourceAcls, error) {
	acls, err := k.admin.ListAcls(sarama.AclFilter{})
	if err != nil {
		return nil, err
	}
	return acls, nil
}

// DeleteUserACLs removes all ACLs for a given user
func (k *kafkaClient) DeleteUserACLs(dn string, patternType v1alpha1.KafkaPatternType) error {
	if patternType == "" {
		patternType = v1alpha1.KafkaPatternTypeDefault
	}
	aclPatternType := AclPatternTypeMapping(patternType)
	matches, err := k.admin.DeleteACL(sarama.AclFilter{
		Principal:                 &dn,
		ResourcePatternTypeFilter: aclPatternType,
		Operation:                 sarama.AclOperationDelete,
		ResourceType:              sarama.AclResourceTopic,
		PermissionType:            sarama.AclPermissionAny,
	}, false)
	if err != nil {
		return err
	}
	for _, x := range matches {
		if x.Err != sarama.ErrNoError {
			return x.Err
		}
	}
	return nil
}

func (k *kafkaClient) createReadACLs(dn string, topic string, patternType sarama.AclResourcePatternType) (err error) {
	if err = k.createCommonACLs(dn, topic, patternType); err != nil {
		return
	}

	// READ on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: patternType,
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

func (k *kafkaClient) createWriteACLs(dn string, topic string, patternType sarama.AclResourcePatternType) (err error) {
	if err = k.createCommonACLs(dn, topic, patternType); err != nil {
		return
	}

	// WRITE on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: patternType,
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
		ResourcePatternType: patternType,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationCreate,
		PermissionType: sarama.AclPermissionAllow,
	})

	return
}

func (k *kafkaClient) createCommonACLs(dn string, topic string, patternType sarama.AclResourcePatternType) (err error) {
	// DESCRIBE on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: patternType,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationDescribe,
		PermissionType: sarama.AclPermissionAllow,
	}); err != nil {
		return
	}

	// DESCRIBE_CONFIGS on topic
	if err = k.admin.CreateACL(sarama.Resource{
		ResourceType:        sarama.AclResourceTopic,
		ResourceName:        topic,
		ResourcePatternType: patternType,
	}, sarama.Acl{
		Principal:      dn,
		Host:           "*",
		Operation:      sarama.AclOperationDescribeConfigs,
		PermissionType: sarama.AclPermissionAllow,
	}); err != nil {
		return
	}
	return
}
