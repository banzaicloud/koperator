## Provisioning Kafka Topics

Creating kafka topics can be done either directly against the cluster with command line utilities, or via the `KafkaTopic` CRD.

Below is an example `KafkaTopic` CR.

```yaml
# topic.yaml
---
apiVersion: kafka.banzaicloud.io/v1alpha1
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

kafkatopic.kafka.banzaicloud.io/example-topic created
```

If you want to update the configuration of the topic after it's been created you can edit the manifest
and run `kubectl apply` again, or you can run `kubectl edit -n kafka kafkatopic example-topic` and then update
the configuration in the editor that gets spawned.

You can increase the partition count for a topic the same way, or for a one-liner using `patch`:

```shell
banzai@cloud:~$ kubectl patch -n kafka kafkatopic example-topic --patch '{"spec": {"partitions": 5}}' --type=merge

kafkatopic.kafka.banzaicloud.io/example-topic patched
```

Operator created Topics are not enforced in any way. From the Kubernetes perspective Kafka Topics are external resources.
We removed the logic which periodically checks the topics state from the operator in release 0.7.0. 
So the operator will not receive any event in case of modification.
It can have unwanted side effects. E.g. if the user deletes a topic created through the CR the operator will recreate it once an update happens on the related CR.
