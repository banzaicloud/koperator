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

Koperator (formerly called Banzai Cloud Kafka Operator) is an operator for managing Apache Kafka on Kubernetes that automates the provisioning, management, and autoscaling of [Apache Kafka](https://kafka.apache.org) clusters deployed to K8s.

## Overview

Apache Kafka is an open-source distributed streaming platform, and some of the main features of the **Koperator** are:

- the provisioning of secure and production ready Kafka clusters
- **fine grained** broker configuration support
- advanced and highly configurable External Access via LoadBalancers using **Envoy**
- graceful Kafka cluster **scaling and rebalancing**
- monitoring via **Prometheus**
- encrypted communication using SSL
- automatic reaction and self healing based on alerts (plugin system, with meaningful default alert plugins) using **Cruise Control**
- graceful rolling upgrade
- advanced topic and user management via CRD

![Koperator architecture](docs/img/kafka-operator-arch.png)

>We took a different approach to what's out there - we believe for a good reason - please read on to understand more about our [design motivations](docs/features.md) and some of the [scenarios](docs/scenarios.md) which were driving us to create Koperator.

**Koperator** is a core part of [Banzai Cloud Supertubes](https://banzaicloud.com/products/supertubes/) (now called Cisco Streaming Data Manager) that helps you create production-ready Apache Kafka clusters on Kubernetes, with scaling, rebalancing, and alerts based self healing. While the Koperator itself is an open-source project, the [Banzai Cloud Supertubes](https://banzaicloud.com/products/supertubes/) product extends the functionality of Koperator with commercial features (for example, built-in monitoring and multiple ways of disaster recovery). Read a [detailed comparison of Supertubes and Koperator](https://banzaicloud.com/docs/supertubes/kafka-operator-supertubes-feature-comparison/).

## Installation

For detailed installation instructions, see the [Banzai Cloud Documentation Page](https://banzaicloud.com/docs/supertubes/kafka-operator/install-kafka-operator/).

## Documentation

The documentation of the Koperator project is available at the [Banzai Cloud Documentation Page](https://banzaicloud.com/docs/supertubes/kafka-operator/).

## Support

### Community support

If you encounter problems while using Koperator that the documentation does not address, [open an issue](https://github.com/banzaicloud/koperator/issues) or talk to us on the Banzai Cloud Slack channel [#kafka-operator](https://banzaicloud.com/invite-slack/).

### Commercial support

If you are using Koperator in a production environment and [require commercial support, contact Banzai Cloud](https://banzaicloud.com/contact/), the company backing the development of Koperator.

## Contributing

If you find this project useful, help us:

- Support the development of this project and star this repo! :star:
- If you use Koperator in a production environment, add yourself to the list of production [adopters](https://github.com/banzaicloud/koperator/blob/master/ADOPTERS.md).:metal: <br>
- Help new users with issues they may encounter :muscle:
- Send a pull request with your new features and bug fixes :rocket:

When you are opening a PR to Koperator the first time we will require you to sign a standard CLA. Check out the [developer docs](docs/developer.md).

## License

Copyright (c) 2019 [Cisco Systems, Inc.](https://www.cisco.com) and/or its affiliates

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

The Banzai Cloud name, the Banzai Cloud logo, and all Banzai Cloud trademarks and logos are registered trademarks of Cisco.
