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

package controllers

import (
	"fmt"
	"time"

	"github.com/banzaicloud/kafka-operator/api/v1alpha1"
	"github.com/banzaicloud/kafka-operator/api/v1beta1"
	"github.com/banzaicloud/kafka-operator/pkg/errorfactory"
	"github.com/banzaicloud/kafka-operator/pkg/kafkaclient"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var clusterRefLabel = "kafkaCluster"

func requeueWithError(logger logr.Logger, msg string, err error) (ctrl.Result, error) {
	// Info log the error message and then let the reconciler dump the stacktrace
	logger.Info(msg)
	return ctrl.Result{}, err
}

func reconciled() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func getClusterRefNamespace(ns string, ref v1alpha1.ClusterReference) string {
	clusterNamespace := ref.Namespace
	if clusterNamespace == "" {
		return ns
	}
	return clusterNamespace
}

func clusterLabelString(cluster *v1beta1.KafkaCluster) string {
	return fmt.Sprintf("%s.%s", cluster.Name, cluster.Namespace)
}

func newBrokerConnection(log logr.Logger, client client.Client, cluster *v1beta1.KafkaCluster) (broker kafkaclient.KafkaClient, close func(), err error) {

	// Get a kafka connection
	log.Info(fmt.Sprintf("Retrieving Kafka client for %s/%s", cluster.Namespace, cluster.Name))
	broker, err = kafkaclient.NewFromCluster(client, cluster)
	if err != nil {
		return
	}
	close = func() {
		if err := broker.Close(); err != nil {
			log.Error(err, "Error closing Kafka client")
		} else {
			log.Info("Kafka client closed cleanly")
		}
	}
	return
}

func checkBrokerConnectionError(logger logr.Logger, err error) (ctrl.Result, error) {
	switch err.(type) {
	case errorfactory.BrokersUnreachable:
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(15) * time.Second,
		}, nil
	case errorfactory.BrokersNotReady:
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(15) * time.Second,
		}, nil
	case errorfactory.ResourceNotReady:
		logger.Info("Needed resource for broker connection not found, may not be ready")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Duration(5) * time.Second,
		}, nil
	default:
		return requeueWithError(logger, err.Error(), err)
	}
}
