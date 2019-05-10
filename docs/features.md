# Features

#### Fine Grained Broker Config Support

Kafka is a stateful application. 
The smallest brick in the puzzle is the Broker which is a simple server capable to create a cluster with other Brokers.
Every Broker has his own unique configuration which slightly differs from the others eg.: unique broker ID.

All Kafka on Kubernetes solution uses StatefulSet to create a Kafka Cluster.

With StatefulSet we get:
- Unique Broker ID generated during Pod startup
- Networking between brokers with headless services
- Unique Persistent Volume for Brokers

Using StatefulSet we lost:
- Modify the configuration of an unique Broker
- Remove specific Broker from cluster
- Using multiple Persistent Volume per Brokers

Banzai Cloud's Kafka Operator is the first one to use `simple Pods`, ConfigMaps, PersistentVolumeClaims instead of StatefulSet.
Using resources other then StatefulSet allows us to build an Operator which can intervene better for Kafka.

With BanzaiCloud's operator we can:
- Modify the configuration of an unique Broker
- Remove specific Broker from the cluster
- Use multiple Persistent Volume per Broker

This also means to react on events, which otherwise impossible to do because of the limitations of the StatefulSet.

No more Broker configs which are impossible to read because of cryptic scripts placed inside the container to generate the config runtime.
With Banzai Cloud's Kafka Operator Broker configs are placed in a specific Configmap. 

#### Graceful Kafka Cluster Scaling

Here at Banzai Cloud we know how to operate Kafka at scale but we also know LinkedIn has way more experience with that.
To gracefully scale Kafka cluster we integrated LinkedIn's [Cruise-Control](https://github.com/linkedin/cruise-control) to do the hard work.

#### External Access via LoadBalancer
#### Communication via SSL
#### Monitoring via Prometheus
#### Reacting on Alerts
