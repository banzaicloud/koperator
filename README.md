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

Some of the main features of Koperator are:

- the provisioning of secure and production-ready Kafka clusters
- fine-grained broker-by-broker configuration support
- advanced and highly configurable external access
- graceful Kafka cluster scaling and rebalancing
- detailed Prometheus metrics
- encrypted communication using SSL
- automatic reaction and self-healing based on alerts using [Cruise Control](https://github.com/linkedin/cruise-control)
- graceful rolling upgrades
- advanced topic and user management via Kubernetes Custom Resources
- Cruise Control task management via Kubernetes Custom Resources

## Architecture

Kafka is a stateful application, and the Kafka Broker is a server that can create and form a cluster with other Brokers. Each Broker has its own unique configuration, the most important of which is the unique broker ID.

Most Kubernetes operators that manage Kafka rely on [`StatefulSets`](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) to create a Kafka Cluster.
While StatefulSets provide unique Broker IDs generated during Pod startup, networking between brokers with headless services, and unique Persistent Volumes for Brokers, they have a few restrictions. For example, Broker configurations cannot be modified independently, and a specific Broker cannot be removed from the cluster - a StatefulSet always removes the most recently created Broker. Furthermore, multiple, different Persistent Volumes cannot be used for each Broker.

*Koperator takes a different approach by using simple `Pods`, `ConfigMaps`, and `PersistentVolumeClaims` instead of `StatefulSets`. These resources allow us to build an Operator that is better suited to manage Apache Kafka.*
With Koperator, you can modify the configuration of unique Brokers, remove specific Brokers from clusters, and use multiple Persistent Volumes for each Broker.

If you want to learn more about our design motivations and the scenarios that drove us to create Koperator, please continue reading on our documentation page [here](https://banzaicloud.github.io/koperator-docs/docs/scenarios/).

![Koperator architecture](docs/img/kafka-operator-arch.png)

## Quick start

This quick start guide will walk you through the process of deploying Koperator on an existing Kubernetes cluster and provisioning a Kafka cluster using its custom resources.

### Prerequisites

To complete this guide, you will need a Kubernetes cluster (with a suggested minimum of 6 vCPUs and 8 GB RAM). You can run the cluster locally using Kind or Minikube.

> The quick start will help you set up a functioning Kafka cluster on Kubernetes. However, it does not include guidance on the installation of Prometheus and cert-manager, which are necessary for some of the more advanced functionality.

#### Install ZooKeeper

The version of Kafka that is installed by the operator requires Apache ZooKeeper. You'll need to deploy a ZooKeeper cluster if you don’t already have one.

1. Install ZooKeeper using [Pravega’s Zookeeper Operator](https://github.com/pravega/zookeeper-operator).

```
helm install zookeeper-operator --repo https://charts.pravega.io zookeeper-operator --namespace=zookeeper --create-namespace
```

2. Create a ZooKeeper cluster.

```
kubectl create -f - <<EOF
apiVersion: zookeeper.pravega.io/v1beta1
kind: ZookeeperCluster
metadata:
    name: zookeeper-server
    namespace: zookeeper
spec:
    replicas: 1
    persistence:
        reclaimPolicy: Delete
EOF
```

3. Verify that ZooKeeper has been deployed.

```
> kubectl get pods -n zookeeper

NAME                                         READY   STATUS    RESTARTS   AGE
zookeeper-server-0                           1/1     Running   0          27m
zookeeper-operator-54444dbd9d-2tccj          1/1     Running   0          28m
```

### Install Koperator

You can deploy Koperator using a Helm chart. Complete the following steps.

1. Install the Koperator `CustomResourceDefinition` resources (adjust the version number to the Koperator release you want to install). This is performed in a separate step to allow you to uninstall and reinstall Koperator without deleting your already installed custom resources.

```
kubectl create --validate=false -f https://github.com/banzaicloud/koperator/releases/download/v0.25.1/kafka-operator.crds.yaml
```

2. Install Koperator into the `kafka` namespace:

```
helm install kafka-operator --repo https://kubernetes-charts.banzaicloud.com kafka-operator --namespace=kafka --create-namespace
```

3. Create the Kafka cluster using the `KafkaCluster` custom resource. The quick start uses a minimal custom resource, but there are other examples in the same directory.

```
kubectl create -n kafka -f https://raw.githubusercontent.com/banzaicloud/koperator/master/config/samples/simplekafkacluster.yaml
```

4. Verify that the Kafka cluster has been created.

```
> kubectl get pods -n kafka

kafka-0-nvx8c                             1/1     Running   0          16m
kafka-1-swps9                             1/1     Running   0          15m
kafka-2-lppzr                             1/1     Running   0          15m
kafka-cruisecontrol-fb659b84b-7cwpn       1/1     Running   0          15m
kafka-operator-operator-8bb75c7fb-7w4lh   2/2     Running   0          17m
```

### Test Kafka cluster

To test the Kafka cluster let's create a topic and send some messages.

1. You can use the `KafkaTopic` CR to create a topic called `my-topic`:

```
kubectl create -n kafka -f - <<EOF
apiVersion: kafka.banzaicloud.io/v1alpha1
kind: KafkaTopic
metadata:
    name: my-topic
spec:
    clusterRef:
        name: kafka
    name: my-topic
    partitions: 1
    replicationFactor: 1
    config:
        "retention.ms": "604800000"
        "cleanup.policy": "delete"
EOF
```

2. If SSL encryption is disabled for Kafka, you can use the following commands to send and receive messages within a Kubernetes cluster.

To send messages, run this command and type your test messages:

```
kubectl -n kafka run kafka-producer -it --image=ghcr.io/banzaicloud/kafka:2.13-3.1.0 --rm=true --restart=Never -- /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka-headless:29092 --topic my-topic
```

To receive messages, run the following command:

```
kubectl -n kafka run kafka-consumer -it --image=ghcr.io/banzaicloud/kafka:2.13-3.1.0 --rm=true --restart=Never -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka-headless:29092 --topic my-topic --from-beginning
```

## Documentation

For detailed documentation on the Koperator project, see the [Koperator documentation website](https://banzaicloud.github.io/koperator-docs/).

## Issues and contributions

We use GitHub to track issues and accept contributions.
If you would like to raise an issue or open a pull request, please refer to our [contribution guide](./CONTRIBUTING.md).

If you use Koperator in a production environment, we encourage you to add yourself to the list of production [adopters](https://github.com/banzaicloud/koperator/blob/master/ADOPTERS.md).

## Community

Find us on [Slack](https://eti.cisco.com/slack) for more fun about Kafka on Kubernetes!

<a href="https://github.com/banzaicloud/koperator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=banzaicloud/koperator&max=30" />
</a>

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
