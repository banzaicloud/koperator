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

For detailed installation instructions, see the [Banzai Cloud Documentation Page](https://banzaicloud.com/docs/supertubes/kafka-operator/install-kafka-operator/).

# TODO

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
