package kafkautil

import (
	"github.com/Shopify/sarama"
	v1alpha1 "github.com/banzaicloud/kafka-operator/api/v1alpha1"
)

func (k *kafkaClient) GetCA() (name string, cakind string) {
	return k.opts.IssueCA, k.opts.IssueCAKind
}

func (k *kafkaClient) CreateUserACLs(accessType v1alpha1.KafkaAccessType, dn string, topic string) error {
	switch accessType {
	case v1alpha1.KafkaAccessTypeRead:
		return k.createReadACLs(dn, topic)
	case v1alpha1.KafkaAccessTypeWrite:
		return k.createWriteACLs(dn, topic)
	default:
		return nil
	}
}

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
