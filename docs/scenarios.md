As highlighted in the [features](/features.md) section, we removed the reliance on StatefulSet, we dig deep and write bunch of code for the operator to allow scenarios as (this is by far not a complete list, get in touch if you have a requirement, question or would like to talk about the operator).


### Vertical capacity scaling

We've seen many times that horizontaly scaling the cluster is not an option (increase resources). When **only one Broker is throttling** and needs more CPU or requires more disks (because it handles more partitions than the others), the StatefulSet based solution can't help as it does not distinguishes between replicas specification. Handling a case as such requires *unique* Broker configuration. If we need to add a new disk to a unique Brokew, with a StatefulSet based solution we will be wasting disk (and money) as we can't add a disk only to a unique Broker, the StatefulSet will add a disk to all replicas.

With the Banzai Cloud operator adding a new disk to any odf the Brokers is just a simple CR configuration change. Same, any per Broker related configuration can be done by a Broker by Broker case.

### Unhandled error happens with Broker #1 in a 3 Broker cluster

In case of an error happens to a Broker (#1) we would like to handle it without disrupting the other Brokers. Maybe we would like to temporarily remove this Broker (#1) from the cluster and fix it's state, reconcile the node serving the node or reconfiguring the Broker with another configuration? Again, using StatefulSet we are losing the ability to remove a specific Broker from the cluster. StatefulSet only supports a field name replica which determines how many replicas should the application use. In case of a downscale this number can be lowered, however this means that Kubernetes will remove the last added Pod (Broker #3) from the cluster - which in this example happened to functional actually quite well. To remove the 1st Broker from the cluster we need to lower this number in a 3 broker cluster to 1. This will cause a state where only one Broker is alive and we are killing brokers which handled traffic without any issue. The Banzai Cloud Kafka operator supports removing specific brokers without disturbing traffic in the cluster.

### Fine grained Broker config support

Apache Kafka is a stateful application, where Brokers create/form a cluster with other Brokers. Every Broker can have his own unique configuration (we support heterogenous environments, no nodes supposed to be the same, act the same or have the same specification - from infrastructure up to the Brokers Envoy configuratrion). Kafka has lots of Broker configs which can be used to fine tune a specific broker, and we did not wanted to limit this to ALL Brokers in a StatefulSet. We support unique Broker configs.

*With these three scenarios mentioned above we decided to not use StatefulSet in our Kafka Operator; instead we decied to rely on Pods, PVCs and ConfigMaps. We believe StatefulSet is very convenient to start with, and handles 80% of the scenarios but introduces huge limitations when running Kafka on Kubernetes in production.* 

### Monitoring based control

Using monitoring is essential for any application, and all relevant information about Kafka should be published to a monitoring solution. In case of Kubernetes the de facto solution is Prometheus, which supports configuring alerts based on consumed metrics. We wanted to build a solution which is standards based (Prometheus and Alert Manager) and can handle and react to alerts automatically, thus human operator don’t have to. The Banzai Cloud Kafka operator supports alert based Kafka cluster management.

### LinkedIn's Cruise Control 

We have a large experience in both operating Kafka and Kubernetes at scale, however we believe that LinkedIn know how to operaste Kafka slightly better. They have built a tool - Cruise Control, to operate their Kafka infrastructure, thus we wanted to build an operator which **handles the infrastructure but does not reinvent the wheel when comes to operating Kafka**.  We didn't want to redevelop already proven and working concepts, we wanted to create an operator which leverages our deep Kubernetes expertise (at the end we have built a CNCF certified Kubernetes distribution, PKE and a hybrid cloud container management platform, Pipeline) by handling all infrastructure related issues - for how we think it is best for Kafka. We believe managing Kafka is a different issue and it already has some neat tools which are standards in the industry, thus we took LinkedIn's Cruise-Control and integrated with the operator.
