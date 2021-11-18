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

package certmanagerpki

import (
	"crypto/tls"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/koperator/pkg/util"
	pkicommon "github.com/banzaicloud/koperator/pkg/util/pki"
)

// GetControllerTLSConfig creates a TLS config from the user secret created for
// cruise control and manager operations
func (c *certManager) GetControllerTLSConfig() (*tls.Config, error) {
	defaultSecretName := fmt.Sprintf(pkicommon.BrokerControllerTemplate, c.cluster.Name)
	return util.GetControllerTLSConfig(c.client, types.NamespacedName{Name: defaultSecretName, Namespace: c.cluster.Namespace})
}
