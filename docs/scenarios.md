## Scenarios

As highlighted in the featiures section, we removed the reliance on StatefulSet, we dig deep and write bunch of code for the operator to allow scenarios as the following ones (this is by far not a complete list, get in touch if you have a requirement, question or would like to talk about the operator).

#### Vertical capacity scaling

We've encountered many situations in which the horizontal scaling of a cluster is impossible. When **only one Broker is throttling** and needs more CPU or requires additional disks (because it handles the most partitions), a StatefulSet-based solution is useless, since it does not distinguishes between replicas' specifications. The handling of such a case requires *unique* Broker configurations. If we need to add a new disk to a unique Broker, with a StatefulSet-based solution, we waste a lot of disk space (and money); since it can't add a disk to a specific Broker, the StatefulSet adds one to each replica.

With the [Banzai Cloud Kafka operator](https://github.com/banzaicloud/kafka-operator), adding a new disk to any Broker is as easy as changing a CR configuration. Similarly, any Broker-specific configuration can be done on a Broker by Broker basis.

### An unhandled error with Broker #1 in a three Broker cluster

In the event of an error with Broker #1, we want to handle it without disrupting the other Brokers. Maybe we would like to temporarily remove this Broker from the cluster, and fix its state, reconciling the node that serves the node, or maybe reconfigure the Broker using a new configuration. Again, when using StatefulSet, we lose the ability to remove specific Brokers from the cluster. StatefulSet only supports a field name replica that determines how many replicas an application should use. If there's a downscale/removal, this number can be lowered, however, this means that Kubernetes will remove the most recently added Pod (Broker #3) from the cluster - which, in this case, happens to suit our purposes quite well. To remove the #1 Broker from the cluster, we need to lower the number of brokers in the cluster from three to one. This will cause a state in which only one Broker is live, while we kill the brokers that handle traffic; the Banzai Cloud Kafka operator supports removing specific brokers without disrupting traffic in the cluster.

### Fine grained Broker config support

Apache Kafka is a stateful application, where Brokers create/form a cluster with other Brokers. Every Broker is uniquely configurable (we support heterogenous environments, in which no nodes are the same, act the same or have the same specifications - from the infrastructure up through the Brokers' Envoy configuration). Kafka has lots of Broker configs, which can be used to fine tune specific brokers, and we did not want to limit these to ALL Brokers in a StatefulSet. We support unique Broker configs.

*In each of the three scenarios lister above, we decided to not use StatefulSet in our Kafka Operator, relying, instead, on Pods, PVCs and ConfigMaps. We believe StatefulSet is very convenient starting point, as it handles roughly 80% of scenarios but introduces huge limitations when running Kafka on Kubernetes in production.*

### Monitoring based control

Use of monitoring is essential for any application, and all relevant information about Kafka should be published to a monitoring solution. When using Kubernetes, the de facto solution is Prometheus, which supports configuring alerts based on previously consumed metrics. We wanted to build a standards-based solution (Prometheus and Alert Manager) that could handle and react to alerts automatically, so human operators wouldn't have to. The Banzai Cloud Kafka operator supports alert-based Kafka cluster management.

### LinkedIn's Cruise Control

We have a lot of experience in operating both Kafka and Kubernetes at scale. However, we believe that LinkedIn knows how to operate Kafka even better than we do. They built a tool, called Cruise Control, to operate their Kafka infrastructure, and we wanted to build an operator which **handled the infrastructure but did not reinvent the wheel insofar as operating Kafka**. We didn't want to redevelop proven concepts, but wanted to create an operator which leveraged our deep Kubernetes expertise (after all, we've already built a CNCF certified Kubernetes distribution, [PKE](https://github.com/banzaicloud/pke) and a hybrid cloud container management platform, [Pipeline](https://github.com/banzaicloud/pipeline)) by handling all Kafka infrastructure related issues in the way we thought best. We believe managing Kafka is a separate issue, for which there already exist some unique tools and solutions that are standard across the industry, so we took LinkedIn's Cruise Control and integrated it with the operator.


