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

package v1alpha1

import (
	"strings"
	"time"

	"github.com/banzaicloud/koperator/api/util"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// default certificate duration if kafkauser.spec.expirationSeconds is not set
	defaultCertificateDuration = time.Hour * 24 * 90
	// CertManagerSignerNamePrefix is acceptable pki backend signerName prefix for cert-manager
	CertManagerSignerNamePrefix string = "clusterissuers.cert-manager.io"
)

// KafkaUserSpec defines the desired state of KafkaUser
// +k8s:openapi-gen=true
type KafkaUserSpec struct {
	// secretName is used as the name of the K8S secret that contains the certificate of the KafkaUser. SecretName should be unique inside the namespace where KafkaUser is located.
	SecretName string           `json:"secretName"`
	ClusterRef ClusterReference `json:"clusterRef"`
	// Annotations defines the annotations placed on the certificate or certificate signing request object
	Annotations    map[string]string `json:"annotations,omitempty"`
	DNSNames       []string          `json:"dnsNames,omitempty"`
	TopicGrants    []UserTopicGrant  `json:"topicGrants,omitempty"`
	IncludeJKS     bool              `json:"includeJKS,omitempty"`
	CreateCert     *bool             `json:"createCert,omitempty"`
	PKIBackendSpec *PKIBackendSpec   `json:"pkiBackendSpec,omitempty"`
	// expirationSeconds is the requested duration of validity of the issued certificate.
	// The minimum valid value for expirationSeconds is 3600 i.e. 1h.
	// When it is not specified the default validation duration is 90 days
	// +optional
	// +kubebuilder:validation:Minimum=3600
	ExpirationSeconds *int32 `json:"expirationSeconds,omitempty"`
}

type PKIBackendSpec struct {
	IssuerRef *cmmeta.ObjectReference `json:"issuerRef,omitempty"`
	// +kubebuilder:validation:Enum={"cert-manager","k8s-csr"}
	PKIBackend string `json:"pkiBackend"`
	// SignerName indicates requested signer, and is a qualified name.
	SignerName string `json:"signerName,omitempty"`
}

// UserTopicGrant is the desired permissions for the KafkaUser
type UserTopicGrant struct {
	TopicName string `json:"topicName"`
	// +kubebuilder:validation:Enum={"read","write"}
	AccessType KafkaAccessType `json:"accessType"`
	// +kubebuilder:validation:Enum={"literal","match","prefixed","any"}
	PatternType KafkaPatternType `json:"patternType,omitempty"`
}

// KafkaUserStatus defines the observed state of KafkaUser
// +k8s:openapi-gen=true
type KafkaUserStatus struct {
	State UserState `json:"state"`
	ACLs  []string  `json:"acls,omitempty"`
}

// KafkaUser is the Schema for the kafka users API
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type KafkaUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaUserSpec   `json:"spec,omitempty"`
	Status KafkaUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaUserList contains a list of KafkaUser
type KafkaUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaUser{}, &KafkaUserList{})
}

func (spec *KafkaUserSpec) GetIfCertShouldBeCreated() bool {
	if spec.CreateCert != nil {
		return *spec.CreateCert
	}
	return true
}

// GetAnnotations returns Annotations to use for certificate or certificate signing request object
func (spec *KafkaUserSpec) GetAnnotations() map[string]string {
	return util.CloneMap(spec.Annotations)
}

// ValidateAnnotations checks if certificate signing request annotations are valid
func (spec *KafkaUserSpec) ValidateAnnotations() error {
	// Validate annotations for cert-manager pki backend signer
	if spec.PKIBackendSpec != nil && strings.Split(spec.PKIBackendSpec.SignerName, "/")[0] == CertManagerSignerNamePrefix {
		certManagerCSRAnnotations := newCertManagerSignerAnnotationsWithValidators()
		err := certManagerCSRAnnotations.validate(spec.GetAnnotations())
		if err != nil {
			return err
		}
	}
	return nil
}

func (spec *KafkaUserSpec) GetExpirationSeconds() int32 {
	if spec.ExpirationSeconds == nil {
		return int32(defaultCertificateDuration.Seconds())
	}
	return *spec.ExpirationSeconds
}
