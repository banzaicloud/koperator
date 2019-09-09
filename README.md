<p align="center"><img src="docs/img/kafka_operator_logo.png" width="160"></p>

<p align="center">

  <a href="https://hub.docker.com/r/banzaicloud/kafka-operator/">
    <img src="https://img.shields.io/docker/cloud/automated/banzaicloud/kafka-operator.svg" alt="Docker Automated build">
  </a>

  <a href="https://circleci.com/gh/banzaicloud/kafka-operator">
    <img src="https://circleci.com/gh/banzaicloud/kafka-operator/tree/master.svg?style=shield" alt="CircleCI">
  </a>

  <a href="https://goreportcard.com/report/github.com/banzaicloud/kafka-operator">
    <img src="https://goreportcard.com/badge/github.com/banzaicloud/kafka-operator" alt="Go Report Card">
  </a>

  <a href="https://github.com/banzaicloud/kafka-operator/">
    <img src="https://img.shields.io/badge/license-Apache%20v2-orange.svg" alt="license">
  </a>

</p>

# Kafka-Operator

The Banzai Cloud Kafka operator is a Kubernetes operator to automate provisioning, management, autoscaling and operations of [Apache Kafka](https://kafka.apache.org) clusters deployed to K8s.

## Overview

Apache Kafka is an open-source distributed streaming platform, and some of the main features of the **Kafka-operator** are:

- the provisioning of secure and production ready Kafka clusters
- **fine grained** broker configuration support
- advanced and highly configurable External Access via LoadBalancers using **Envoy**
- graceful Kafka cluster **scaling and rebalancing**
- monitoring via **Prometheus**
- encrypted communication using SSL
- automatic reaction and self healing based on alerts (plugin system, with meaningful default alert plugins) using **Cruise Control**

![Kafka-operator architecture](docs/img/kafka-operator-arch.png)

>We took a different approach to what's out there - we believe for a good reason - please read on to understand more about our [design motivations](docs/features.md) and some of the [scenarios](docs/scenarios.md) which were driving us to create the Banzai Cloud Kafka operator.

## Motivation

At [Banzai Cloud](https://banzaicloud.com) we are building a Kubernetes distribution, [PKE](https://github.com/banzaicloud/pke), and a hybrid-cloud container management platform, [Pipeline](https://github.com/banzaicloud/pipeline), that operate Kafka clusters (among other types) for our customers. Apache Kafka predates Kubernetes and was designed mostly for `static` on-premise environments. State management, node identity, failover, etc all come part and parcel with Kafka, so making it work properly on Kubernetes and on an underlying dynamic environment can be a challenge.

There are already several approaches to operating Kafka on Kubernetes, however, we did not find them appropriate for use in a highly dynamic environment, nor capable of meeting our customers' needs. At the same time, there is substantial interest within the Kafka community for a solution which enables Kafka on Kubernetes, both in the open source and closed source space.

- [Helm chart](https://github.com/confluentinc/cp-helm-charts/tree/master/charts/cp-kafka)
- [Yaml files](https://github.com/Yolean/kubernetes-kafka)
- [Strimzi Kafka Operator](https://github.com/strimzi/strimzi-kafka-operator)
- Confluent operator

Join us as we take a deep dive into some of the details of the most popular pre-existing solutions, as well as our own:

|               | Banzai Cloud | Krallistic | Strimzi | Confluent|
| ------------- | ------------ | ------------ | ------------ | ------------ |
| Open source  | Apache 2 | Apache 2|Apache 2| No|
| Fine grained broker config support  | Yes (learn more) | Limited via StatefulSet|Limited via StatefulSet|Limited via StatefulSet|
| Fine grained broker volume support  | Yes (learn more) | Limited via StatefulSet|Limited via StatefulSet|Limited via StatefulSet|
| Monitoring  | Yes | Yes|Yes|Yes|
| Encryption using SSL  | Yes | Yes|Yes|Yes|
| Rolling Update  | Work in progress | No |No|Yes|
| Cluster external accesses  | Envoy (single LB) | Nodeport |Nodeport or LB/broker|Yes (N/A)|
| User Management via CRD  | Yes | No |Yes|No|
| Topic management via CRD  | Yes | No |Yes|No|
| Reacting to Alerts| Yes (Prometheus + Cruise Control | No |No|No|
| Graceful Cluster Scaling (up and down)| Yes (using Cruise Control) | No |No|Yes|

*-if you find any of this information inaccurate, please let us know, and we'll fix it*

>We took a different approach to what's out there - we believe for a good reason - please read on to understand more about our [design motivations](docs/features.md) and some of the [scenarios](docs/scenarios.md) which were driving us to create the Banzai Cloud Kafka operator.

Finally, our motivation is to build an open source solution and a community which drives the innovation and features of this operator. We are long term contributors and active community members of both Apache Kafka and Kubernetes, and we hope to recreate a similar community around this operator.  

If you are willing to kickstart your managed Apache Kafka experience on 5 cloud providers, on-premise or hybrid environments, check out the free developer beta:
<p align="center">
  <a href="https://beta.banzaicloud.io">
  <img src="https://camo.githubusercontent.com/a487fb3128bcd1ef9fc1bf97ead8d6d6a442049a/68747470733a2f2f62616e7a6169636c6f75642e636f6d2f696d672f7472795f706970656c696e655f627574746f6e2e737667">
  </a>
</p>

## Installation

The operator installs the 2.1.0 version of Apache Kafka, and can run on Minikube v0.33.1+ and Kubernetes 1.12.0+.

As a pre-requisite it needs a Kubernetes cluster (you can create one using [Pipeline](https://github.com/banzaicloud/pipeline)). Also, Kafka requires Zookeeper so you need to first have a Zookeeper cluster if you don't already have one.

The operator also uses `cert-manager` for issuing certificates to users and brokers, so you'll need to have it setup in case you haven't already.

> We believe in the `separation of concerns` principle, thus the Kafka operator does not install nor manage Zookeeper or cert-manager. If you would like to have a fully automated and managed experience of Apache Kafka on Kubernetes please try it with [Pipeline](https://github.com/banzaicloud/pipeline).

#### Install cert-manager

```bash
# Add the jetstack helm repo
helm repo add jetstack https://charts.jetstack.io

# pre-create cert-manager namespace and CRDs per their installation instructions
kubectl create ns cert-manager
kubectl label namespace cert-manager certmanager.k8s.io/disable-validation=true
kubectl apply -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.10/deploy/manifests/00-crds.yaml

# Install cert-manager into the cluster
# --set webhook.enabled=false may not be required for you, but avoids issues with
# certificates not being able to be issued due to the webhook not working.
helm install --name cert-manager --namespace cert-manager --version v0.10.0 --set webhook.enabled=false jetstack/cert-manager
```

##### Install Zookeeper

To install Zookeeper we recommend using the [Pravega's Zookeeper Operator](https://github.com/pravega/zookeeper-operator).
You can deploy Zookeeper by using the Helm chart.

```bash
helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
helm install --name zookeeper-operator --namespace=zookeeper banzaicloud-stable/zookeeper-operator
kubectl create --namespace zookeeper -f - <<EOF
apiVersion: zookeeper.pravega.io/v1beta1
kind: ZookeeperCluster
metadata:
  name: example-zookeepercluster
  namespace: zookeeper
spec:
  replicas: 3
EOF

```

### Installation

We recommend to use a **custom StorageClass** to leverage the volume binding mode `WaitForFirstConsumer`

```bash
apiVersion: storage.k8s.io/v1
kind: StorageClass
  name: exampleStorageclass
parameters:
  type: pd-standard
provisioner: kubernetes.io/gce-pd
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```
> Remember to set your Kafka CR properly to use the newly created StorageClass.

1. Set `KUBECONFIG` pointing towards your cluster
2. Run `make deploy` (deploys the operator in the `kafka` namespace into the cluster)
3. Set your Kafka configurations in a Kubernetes custom resource (sample: `config/samples/banzaicloud_v1alpha1_kafkacluster.yaml`) and run this command to deploy the Kafka components:

```bash
# Add your zookeeper svc name to the configuration
kubectl create -n kafka -f config/samples/example-secret.yaml
kubectl create -n kafka -f config/samples/banzaicloud_v1alpha1_kafkacluster.yaml
```

> In this case you have to install Prometheus with proper configuration if you want the Kafka-Operator to react to alerts. Again, if you need Prometheus and would like to have a fully automated and managed experience of Apache Kafka on Kubernetes please try it with [Pipeline](https://github.com/banzaicloud/pipeline).


### Easy way: installing with Helm

Alternatively, if you are using Helm, you can deploy the operator using a Helm chart [Helm chart](https://github.com/banzaicloud/kafka-operator/tree/master/charts):

```bash
helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com/
helm install --name=kafka-operator --namespace=kafka banzaicloud-stable/kafka-operator -f config/samples/example-prometheus-alerts.yaml
# Add your zookeeper svc name to the configuration
kubectl create -n kafka -f config/samples/example-secret.yaml
kubectl create -n kafka -f config/samples/banzaicloud_v1alpha1_kafkacluster.yaml
```

> In this case Prometheus will be installed and configured properly for the Kafka-Operator.

## Test Your Deployment

For simple test code please check out the [test docs](docs/test.md)

For a more in-depth view at using SSL and the `KafkaUser` CRD see the [SSL docs](docs/ssl.md)

For creating topics via with `KafkaTopic` CRD there is an example and more information in the [topics docs](docs/topics.md)

## Development

Check out the [developer docs](docs/developer.md).

## Features

Check out the [supported features](docs/features.md).

## Issues, feature requests and roadmap

Please note that the Kafka operator is constantly under development and new releases might introduce breaking changes. We are striving to keep backward compatibility as much as possible while adding new features at a fast pace. Issues, new features or bugs are tracked on the projects [GitHub page](https://github.com/banzaicloud/kafka-operator/issues) - please feel free to add yours!

To track some of the significant features and future items from the roadmap please visit the [roadmap doc](docs/roadmap.md).

## Contributing

If you find this project useful here's how you can help:

- Send a pull request with your new features and bug fixes
- Help new users with issues they may encounter
- Support the development of this project and star this repo!

When you are opening a PR to Kafka operator the first time we will require you to sign a standard CLA.

## Community

If you have any questions about the Kafka operator, and would like to talk to us and the other members of the Banzai Cloud community, please join our **#kafka-operator** channel on [Slack](https://slack.banzaicloud.io/).

## License

Copyright (c) 2019 [Banzai Cloud, Inc.](https://banzaicloud.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
