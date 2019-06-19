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

package k8sutil

import (
	"context"
	"fmt"

	banzaicloudv1alpha1 "github.com/banzaicloud/kafka-operator/pkg/apis/banzaicloud/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateStatus(c client.Client, brokerId int32, cluster *banzaicloudv1alpha1.KafkaCluster, status banzaicloudv1alpha1.BrokerState, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	brokerStatus := map[int32]banzaicloudv1alpha1.BrokerState{}

	for k, v := range cluster.Status.BrokersState {
		brokerStatus[k] = v
	}

	brokerStatus[brokerId] = status
	cluster.Status.BrokersState = brokerStatus
	err := c.Status().Update(context.Background(), cluster)
	if errors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !errors.IsConflict(err) {
			return emperror.Wrapf(err, "could not update Kafka cluster broker %d state to '%s'", brokerId, status)
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return emperror.Wrap(err, "could not get config for updating status")
		}
		brokerStatus[brokerId] = status
		cluster.Status.BrokersState = brokerStatus
		err = c.Status().Update(context.Background(), cluster)
		if errors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return emperror.Wrapf(err, "could not update Kafka clusters broker %d state to '%s'", brokerId, status)
		}
	}
	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info("Kafka cluster state updated", "status", status)
	return nil
}

// DeleteStatus deletes the given broker state from the CR
func DeleteStatus(c client.Client, brokerId int32, cluster *banzaicloudv1alpha1.KafkaCluster, logger logr.Logger) error {
	typeMeta := cluster.TypeMeta

	brokerStatus := cluster.Status.BrokersState

	delete(brokerStatus, brokerId)

	cluster.Status.BrokersState = brokerStatus

	err := c.Status().Update(context.Background(), cluster)
	if errors.IsNotFound(err) {
		err = c.Update(context.Background(), cluster)
	}
	if err != nil {
		if !errors.IsConflict(err) {
			return emperror.Wrapf(err, "could not delete Kafka cluster broker %d state ", brokerId)
		}
		err := c.Get(context.TODO(), types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		}, cluster)
		if err != nil {
			return emperror.Wrap(err, "could not get config for updating status")
		}
		brokerStatus = cluster.Status.BrokersState

		delete(brokerStatus, brokerId)

		cluster.Status.BrokersState = brokerStatus
		err = c.Status().Update(context.Background(), cluster)
		if errors.IsNotFound(err) {
			err = c.Update(context.Background(), cluster)
		}
		if err != nil {
			return emperror.Wrapf(err, "could not delete Kafka clusters broker %d state ", brokerId)
		}
	}

	// update loses the typeMeta of the config that's used later when setting ownerrefs
	cluster.TypeMeta = typeMeta
	logger.Info(fmt.Sprintf("Kafka broker %d state deleted", brokerId))
	return nil
}
