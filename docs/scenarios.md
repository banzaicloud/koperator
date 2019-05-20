
### Vertical capacity scaling:

Sometimes horizontal scaling the cluster is not an option.  Only one broker is throttling and needs more CPU or 
it requires more disk because it handles more partitions than others. StatefulSet backed solutions does not distinguishes replicas specification. It means creating
truly unique brokers is not achievable. Adding one more persistent volume to the broker can not be done without wasting as many volumes as the number of brokers. StatefulSet will attach a new volume for every replica. We wanted a solution where adding a new disk to the broker is just a simple configuration change, and does not triggers unnecessary volume creates.

### Unhandled error happens with broker 1 in 3 broker cluster:
The error should be handled somehow, preferably with any disruption to the other brokers.
We need to remove this broker from the cluster and fix it's state.  In case of infrastructure error maybe a new node should be selected, or if something
does not adds up with a config a new configuration should be made. Using StatefulSet however we are losing the ability to remove a specific broker from the cluster. 
Statefulset only supports a field name replica which determines how many replicas should the application use. In case of downscale this number can be lowered.
This means Kubernetes will remove the last added Pod from the cluster which happened to functional well. To remove the 1st broker from the cluster we need to lower 
this number in a 3 broker cluster to 1. This causes a state where only one broker is alive and we need to kill a broker which handled traffic without any issue.
We wanted a solution where removing a specific broker is feasible. 

### Fine grained Broker config support:
Kafka is a stateful application. The smallest brick is the Broker which is a simple server capable to create a cluster with other Brokers. 
Every Broker has his own unique configuration which slightly differs from the others eg.: unique broker ID.  In fact, there are lot of Kafka broker configs which can be used to
fine tune a specific broker. We wanted a solution which allows this.  StatefulSet narrows this configuration because all the created replicas inherits the basic configuration. 

With the three scenarios mentioned above we decided to not use StatefulSet in our Operator instead we should rely on Pods, PVCs and ConfigMaps. We think StatefulSet handles the 80% of the scenarios but there are ones where it introduces limitation which cannot be handled.

### Monitoring based control:
Using monitoring is essential for any application. All relevant information about apps are published to a monitoring solution. In case of Kubernetes the de facto solution is Prometheus. Prometheus supports configuring alerts based on consumed metrics. We looked for a solution which can handle the most urgent alerts automatically so the human operator don’t have to. Our Operator supports alert based kafka cluster management.

### Operator which handles Infra but does not reinvent the wheel operating Kafka:
Here at BanzaiCloud we operate Kafka at Scale but we know there are companies but better expertise. We are Kubernetes experts with some Big Data background. We don’t want to redevelop the already working concepts we wanted to create an operator which leverages our Kubernetes expertise by handling the infrastructure how we think is the best for Kafka. We think managing Kafka is a different issue which already has some neat tool which are standards in the industry. So we took LinkedIns Cruise-Control a goal based Kafka manager tool and integrated inside our Operator.
