<p align="center">

![Koperator](https://img.shields.io/github/v/release/banzaicloud/koperator?label=Koperator&sort=semver)
![Released](https://img.shields.io/github/release-date/banzaicloud/koperator?label=Released)
![License](https://img.shields.io/github/license/banzaicloud/koperator?label=License)
![Go version (latest release)](https://img.shields.io/github/go-mod/go-version/banzaicloud/koperator/v0.22.0)

</p>

---

<p align="center">

![Go version](https://img.shields.io/github/go-mod/go-version/banzaicloud/koperator/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/banzaicloud/koperator)](https://goreportcard.com/report/github.com/banzaicloud/koperator)
![CI](https://img.shields.io/github/actions/workflow/status/banzaicloud/koperator/ci.yml?branch=master&label=CI)
![Image](https://img.shields.io/github/actions/workflow/status/banzaicloud/koperator/docker.yml?branch=master&label=Image)
![Image (perf test)](https://img.shields.io/github/actions/workflow/status/banzaicloud/koperator/docker_perf_test_load.yml?branch=master&label=Image%20%28perf%20test%29)
![Helm chart](https://img.shields.io/github/actions/workflow/status/banzaicloud/koperator/helm.yml?branch=master&label=Helm%20chart)

</p>

# Koperator

Koperator is an open-source operator that automates the provisioning, management, and autoscaling of Apache Kafka clusters on Kubernetes. 
Unlike other solutions that rely on StatefulSets, Koperator has been built with a unique architecture that provides greater flexibility and functionality for managing Apache Kafka. This architecture allows for fine-grained configuration and management of individual brokers.

Some of the main features of **Koperator** are:

- the provisioning of secure and production ready Kafka clusters
- fine grained broker-by-broker configuration support
- advanced and highly configurable external access
- graceful Kafka cluster scaling and rebalancing
- detailed Prometheus metrics
- encrypted communication using SSL
- automatic reaction and self healing based on alerts using [Cruise Control](https://github.com/linkedin/cruise-control)
- graceful rolling upgrades
- advanced topic and user management via Kubernetes Custom Resources
- Cruise Control task management via Kubernetes Custom Resources

![Koperator architecture](docs/img/kafka-operator-arch.png)

## Architecture

Kafka is a stateful application. The first piece of the puzzle is the Broker, which is a simple server capable of creating/forming a cluster with other Brokers. Every Broker has his own unique configuration which differs slightly from all others - the most relevant of which is the unique broker ID.

All Kafka on Kubernetes operators use [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) to create a Kafka Cluster.

How does this look from the perspective of Apache Kafka?

With StatefulSet we get:

- unique Broker IDs generated during Pod startup
- networking between brokers with headless services
- unique Persistent Volumes for Brokers

Using StatefulSet we lose:

- the ability to modify the configuration of unique Brokers
- to remove a specific Broker from a cluster (StatefulSet always removes the most recently created Broker)
- to use multiple, different Persistent Volumes for each Broker

*Koperator uses simple Pods, ConfigMaps, and PersistentVolumeClaims, instead of StatefulSet. Using these resources allows us to build an Operator which is better suited to manage Apache Kafka.*

With the Koperator you can:

- modify the configuration of unique Brokers
- remove specific Brokers from clusters
- use multiple Persistent Volumes for each Broker

Read on to understand more about our design motivations and some of the [scenarios](https://docs.calisti.app/sdm/koperator/scenarios/) which were driving us to create Koperator.

## Quick start

This quick start guides through the process of deploying Koperator on an existing Kubernetes cluster and the provisioning of a Kafka cluster using its custom resources.

### Prerequisites

A Kubernetes cluster (with a suggested minimum of 6 vCPUs and 8 GB RAM) is needed to complete this guide. The cluster can run locally via Kind or Minikube.

> The quick start will get you a functioning Kafka cluster on Kubernetes, but it won't guide you through the installation of Prometheus and cert-manager, that are needed for some of the more advanced functionality.

#### Install ZooKeeper

The version of Kafka that is installed by the operator requires Apache ZooKeeper. You'll need to deploy a ZooKeeper cluster if you don’t already have one.

1. Install ZooKeeper using the [Pravega’s Zookeeper Operator](https://github.com/pravega/zookeeper-operator).

```
helm repo add pravega https://charts.pravega.io
helm repo update
helm install zookeeper-operator --namespace=zookeeper --create-namespace pravega/zookeeper-operator
```

2. Create a ZooKeeper cluster.

```
kubectl create --namespace zookeeper -f - <<EOF
apiVersion: zookeeper.pravega.io/v1beta1
kind: ZookeeperCluster
metadata:
    name: zookeeper
    namespace: zookeeper
spec:
    replicas: 1
EOF
```

3. Verify that ZooKeeper has been deployed.

```
> kubectl get pods -n zookeeper

NAME                                  READY   STATUS    RESTARTS   AGE
zookeeper-0                           1/1     Running   0          27m
zookeeper-operator-54444dbd9d-2tccj   1/1     Running   0          28m
```

### Install Koperator

You can deploy Koperator using a Helm chart. Complete the following steps.

1. Install the Koperator CustomResourceDefinition resources (adjust the version number to the Koperator release you want to install). This is performed in a separate step to allow you to uninstall and reinstall Koperator without deleting your installed custom resources.

```
kubectl create --validate=false -f https://github.com/banzaicloud/koperator/releases/download/v0.23.1/kafka-operator.crds.yaml
```

2. Add the following repository to Helm.

```
helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
helm repo update
```

3. Install Koperator into the kafka namespace:

```
helm install kafka-operator --namespace=kafka --create-namespace banzaicloud-stable/kafka-operator
```

4. Create the Kafka cluster using the `KafkaCluster` custom resource. You can find various examples for the custom resource in the Koperator repository.

```
kubectl create --namespace kafka -f - <<EOF
apiVersion: kafka.banzaicloud.io/v1beta1
kind: KafkaCluster
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: kafka
spec:
  monitoringConfig:
    jmxImage: "ghcr.io/banzaicloud/jmx-javaagent:0.16.1"
  headlessServiceEnabled: true
  zkAddresses:
    - "zookeeper-client.zookeeper:2181"
  propagateLabels: false
  oneBrokerPerNode: false
  clusterImage: "ghcr.io/banzaicloud/kafka:2.13-3.1.0"
  readOnlyConfig: |
    auto.create.topics.enable=false
    cruise.control.metrics.topic.auto.create=true
    cruise.control.metrics.topic.num.partitions=1
    cruise.control.metrics.topic.replication.factor=2
  brokerConfigGroups:
    default:
      storageConfigs:
        - mountPath: "/kafka-logs"
          pvcSpec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 10Gi
      brokerAnnotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9020"
  brokers:
    - id: 0
      brokerConfigGroup: "default"
    - id: 1
      brokerConfigGroup: "default"
    - id: 2
      brokerConfigGroup: "default"
  rollingUpgradeConfig:
    failureThreshold: 1
  listenersConfig:
    internalListeners:
      - type: "plaintext"
        name: "internal"
        containerPort: 29092
        usedForInnerBrokerCommunication: true
      - type: "plaintext"
        name: "controller"
        containerPort: 29093
        usedForInnerBrokerCommunication: false
        usedForControllerCommunication: true
EOF
```

5. Verify that the Kafka cluster has been created.

```
> kubectl get pods -n kafka

kafka-0-nvx8c                             1/1     Running   0          16m
kafka-1-swps9                             1/1     Running   0          15m
kafka-2-lppzr                             1/1     Running   0          15m
kafka-cruisecontrol-fb659b84b-7cwpn       1/1     Running   0          15m
kafka-operator-operator-8bb75c7fb-7w4lh   2/2     Running   0          17m
```

## Documentation

The detailed documentation of the Koperator project is available in the [Cisco Calisti documentation](https://docs.calisti.app/sdm/koperator/).

## Support

### Community support

If you encounter problems while using Koperator that the documentation does not address, [open an issue](https://github.com/banzaicloud/koperator/issues) or talk to us on the Banzai Cloud Slack channel [#kafka-operator](https://banzaicloud.com/invite-slack/).

## Contributing

If you find this project useful, help us:

- Support the development of this project and star this repo! :star:
- If you use Koperator in a production environment, add yourself to the list of production [adopters](https://github.com/banzaicloud/koperator/blob/master/ADOPTERS.md).:metal: <br>
- Help new users with issues they may encounter :muscle:
- Send a pull request with your new features and bug fixes :rocket:

When you are opening a PR to Koperator the first time we will require you to sign a standard CLA. Check out the [developer docs](docs/developer.md).

## License

Copyright (c) 2023 [Cisco Systems, Inc.](https://www.cisco.com) and/or its affiliates

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Trademarks

Apache Kafka, Kafka, and the Kafka logo are either registered trademarks or trademarks of The Apache Software Foundation in the United States and other countries.
