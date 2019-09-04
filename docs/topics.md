## Provisioning Kafka Topics

Creating kafka topics can be done either directly against the cluster with command line utilities, or via the `KafkaTopic` CRD.

Below is an example `KafkaTopic` CR.

```yaml
# topic.yaml
---
apiVersion: banzaicloud.banzaicloud.io/v1alpha1
kind: KafkaTopic
metadata:
  name: example-topic
  namespace: kafka
spec:
  clusterRef:
    name: kafka
  name: example-topic
  partitions: 3
  replicationFactor: 2
  config:
    # For a full list of configuration options, refer to the official documentation.
    # https://kafka.apache.org/documentation/#topicconfigs
    "retention.ms": "604800000"
    "cleanup.policy": "delete"
```

You can apply the above topic with kubectl

```shell
banzai@cloud:~$ kubectl apply -n kafka -f topic.yaml

kafkatopic.banzaicloud.banzaicloud.io/example-topic created
```

If you want to update the configuration of the topic after it's been created you can edit the manifest
and run `kubectl apply` again, or you can run `kubectl edit -n kafka kafkatopic example-topic` and then update
the configuration in the editor that gets spawned.

You can increase the partition count for a topic the same way, or for a one-liner using `patch`:

```shell
banzai@cloud:~$ kubectl patch -n kafka kafkatopic example-topic --patch '{"spec": {"partitions": 5}}' --type=merge

kafkatopic.banzaicloud.banzaicloud.io/example-topic patched
```

The operator will periodically poll kafka for the topic's status and post the data to the CR status.
You can view the status with `describe`.

```shell
banzai@cloud:~$ kubectl describe -n kafka kafkatopic example-topic

Name:         example-topic
Namespace:    kafka
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"banzaicloud.banzaicloud.io/v1alpha1","kind":"KafkaTopic","metadata":{"annotations":{},"name":"example-topic","namespace":"k...
API Version:  banzaicloud.banzaicloud.io/v1alpha1
Kind:         KafkaTopic
Metadata:
  Creation Timestamp:  2019-09-04T22:13:34Z
  Finalizers:
    finalizer.kafkatopics.banzaicloud.banzaicloud.io
  Generation:  3
  Owner References:
    API Version:           banzaicloud.banzaicloud.io/v1alpha1
    Block Owner Deletion:  true
    Controller:            true
    Kind:                  KafkaCluster
    Name:                  kafka
    UID:                   a64f4e80-d88b-4f21-9e08-c141f80fde07
  Resource Version:        8184847
  Self Link:               /apis/banzaicloud.banzaicloud.io/v1alpha1/namespaces/kafka/kafkatopics/example-topic
  UID:                     407964fa-4c45-4431-9478-278045f9ed53
Spec:
  Cluster Ref:
    Name:       kafka
    Namespace:  kafka
  Config:
    cleanup.policy:    delete
    retention.ms:      604800000
  Name:                example-topic
  Partitions:          5
  Replication Factor:  2
Status:
  In Sync Replicas:
    0:  [1 0]
    1:  [0 2]
    2:  [2 1]
    3:  [1 2]
    4:  [2 0]
  Leaders:
    0:  1/kafka-1.kafka.svc.cluster.local:9092
    1:  0/kafka-0.kafka.svc.cluster.local:9092
    2:  2/kafka-2.kafka.svc.cluster.local:9092
    3:  1/kafka-1.kafka.svc.cluster.local:9092
    4:  2/kafka-2.kafka.svc.cluster.local:9092
  Offline Replicas:
  Partition Count:  5
  Replica Counts:
    0:   2
    1:   2
    2:   2
    3:   2
    4:   2
```

The keys in the `Status` maps refer to the partition ID.
