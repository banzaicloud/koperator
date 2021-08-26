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

package v1alpha1

// KafkaAccessType hold info about Kafka ACL
type KafkaAccessType string

// KafkaPatternType hold the Resource Pattern Type of kafka ACL
type KafkaPatternType string

// TopicState defines the state of a KafkaTopic
type TopicState string

// UserState defines the state of a KafkaUser
type UserState string

// ClusterReference states a reference to a cluster for topic/user
// provisioning
type ClusterReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

const (
	// KafkaAccessTypeRead states that a user wants consume access to a topic
	KafkaAccessTypeRead KafkaAccessType = "read"
	// KafkaAccessTypeWrite states that a user wants produce access to a topic
	KafkaAccessTypeWrite KafkaAccessType = "write"
	// Resource pattern types. More info: https://kafka.apache.org/20/javadoc/org/apache/kafka/common/resource/PatternType.html
	KafkaPatternTypeAny      KafkaPatternType = "any"
	KafkaPatternTypeLiteral  KafkaPatternType = "literal"
	KafkaPatternTypeMatch    KafkaPatternType = "match"
	KafkaPatternTypePrefixed KafkaPatternType = "prefixed"
	KafkaPatternTypeDefault  KafkaPatternType = "literal"
	// TopicStateCreated describes the status of a KafkaTopic as created
	TopicStateCreated TopicState = "created"
	// UserStateCreated describes the status of a KafkaUser as created
	UserStateCreated UserState = "created"
	// TLSJKSKeyStore is where a JKS keystore is stored in a user secret when requested
	TLSJKSKeyStore string = "keystore.jks"
	// TLSJKSTrustStore is where a JKS truststore is stored in a user secret when requested
	TLSJKSTrustStore string = "truststore.jks"
	// CoreCACertKey is where ca ceritificates are stored in user certificates
	CoreCACertKey string = "ca.crt"
	// CACertKey is the key where the CA certificate is stored in the operator secrets
	CACertKey string = "caCert"
	// CAPrivateKeyKey stores the private key for the CA
	CAPrivateKeyKey string = "caKey"
	// ClientCertKey stores the client certificate (cruisecontrol/operator usage)
	ClientCertKey string = "clientCert"
	// ClientPrivateKeyKey stores the client private key
	ClientPrivateKeyKey string = "clientKey"
	// PeerCertKey stores the peer certificate (broker certificates)
	PeerCertKey string = "peerCert"
	// PeerPrivateKeyKey stores the peer private key
	PeerPrivateKeyKey string = "peerKey"
	// PasswordKey stores the JKS password
	PasswordKey string = "password"
)
