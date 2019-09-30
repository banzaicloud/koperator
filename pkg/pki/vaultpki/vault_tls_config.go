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

package vaultpki

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	pkicommon "github.com/banzaicloud/kafka-operator/pkg/util/pki"
)

func (v *vaultPKI) GetControllerTLSConfig() (config *tls.Config, err error) {
	config = &tls.Config{}

	vault, err := v.getClient()
	if err != nil {
		return
	}

	// TODO (tinyzimmer): Maybe still just grab from the kubernetes secret we
	// make for cruise control.
	secret, v2, err := getSecret(vault, fmt.Sprintf("secret/%s", fmt.Sprintf(pkicommon.BrokerControllerTemplate, v.cluster.Name)))
	if err != nil {
		err = errorfactory.New(errorfactory.VaultAPIFailure{}, err, "could not fetch controller certificate")
		return
	} else if secret == nil || secret.Data == nil {
		err = errorfactory.New(errorfactory.ResourceNotReady{}, errors.New("not found"), "controller secret is empty")
	}

	cert, err := userCertForData(v2, secret.Data)
	if err != nil {
		err = errorfactory.New(errorfactory.InternalError{}, err, "could not parse controller vault secret")
		return
	}

	x509ClientCert, err := tls.X509KeyPair(cert.Certificate, cert.Key)
	if err != nil {
		err = errorfactory.New(errorfactory.InternalError{}, err, "could not decode controller certificate")
		return
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(cert.CA)

	config.Certificates = []tls.Certificate{x509ClientCert}
	config.RootCAs = rootCAs

	return
}
