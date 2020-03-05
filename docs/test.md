# Test provisioned Kafka Cluster


## Create Topic

Topic creation by default is enabled in Kafka, but if it is configured otherwise, you'll need to first create a topic.

You can use the `KafkaTopic` CRD to make a topic like this:

```bash
cat << EOF | kubectl apply -n kafka -f -
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
EOF
```

*Note: The above will fail if the cluster has not finished provisioning*

To create a sample topic from the CLI you can run the following:

```bash
kubectl -n kafka run kafka-topics -it --image=banzaicloud/kafka:2.13-2.4.0 --rm=true --restart=Never -- /opt/kafka/bin/kafka-topics.sh --zookeeper zookeeper-client.zookeeper:2181 --topic my-topic --create --partitions 1 --replication-factor 1
```

## Send and Receive Messages

### Inside Kubernetes Cluster

#### SSL Disabled

##### Produce Messages

```bash
kubectl -n kafka run kafka-producer -it --image=banzaicloud/kafka:2.13-2.4.0 --rm=true --restart=Never -- /opt/kafka/bin/kafka-console-producer.sh --broker-list kafka-headless:29092 --topic my-topic
```

##### Consume Messages

```bash
kubectl -n kafka run kafka-consumer -it --image=banzaicloud/kafka:2.13-2.4.0 --rm=true --restart=Never -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka-headless:29092 --topic my-topic --from-beginning
```

#### SSL Enabled

To try Kafka guarded by SSL we recommend to use [Kafkacat](https://github.com/edenhill/kafkacat).

> Want to use the java client instead please generate the proper truststore and keystore using the [official docs](https://kafka.apache.org/documentation/#security_ssl).

To use Kafka inside the cluster, create a Pod which contains `Kafkacat`.

The following command will create a `kafka-test` pod in the `kafka` namespace.

```bash
kubectl create -n kafka -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: kafka-test
spec:
  containers:
  - name: kafka-test
    image: solsson/kafkacat
    # Just spin & wait forever
    command: [ "/bin/bash", "-c", "--" ]
    args: [ "while true; do sleep 3000; done;" ]
    volumeMounts:
    - name: sslcerts
      mountPath: "/ssl/certs"
  volumes:
  - name: sslcerts
    secret:
      secretName: kafka-operator-server-cert
EOF
```

Then exec into the container and produce and consume some messages:

```bash
kubectl exec -it -n kafka kafka-test bash
```

##### Produce Messages

```bash
kafkacat -P -b kafka-headless:29092 -t my-topic \
-X security.protocol=SSL \
-X ssl.key.location=/ssl/certs/tls.key \
-X ssl.certificate.location=/ssl/certs/tls.crt \
-X ssl.ca.location=/ssl/certs/ca.crt
```

##### Consume Messages

```bash
kafkacat -C -b kafka-headless:29092 -t my-topic \
-X security.protocol=SSL \
-X ssl.key.location=/ssl/certs/tls.key \
-X ssl.certificate.location=/ssl/certs/tls.crt \
-X ssl.ca.location=/ssl/certs/ca.crt

```

The above will use the certificate provisioned with the cluster for connecting to kafka.
If you'd like to create and use a different user, create a `KafkaUser` CR. An example can be found on the [SSL doc](docs/ssl.md).

### Outside Kubernetes Cluster

We need to get the LoadBalancer address first using:

```bash
export SERVICE_IP=$(kubectl get svc --namespace kafka -o jsonpath="{.status.loadBalancer.ingress[0].ip}" envoy-loadbalancer)

echo $SERVICE_IP

export SERVICE_PORTS=($(kubectl get svc --namespace kafka -o jsonpath="{.spec.ports[*].port}" envoy-loadbalancer))

echo ${SERVICE_PORTS[@]}

# depending on the shell of your choice, arrays may be indexed starting from 0 or 1
export SERVICE_PORT=${SERVICE_PORTS[@]:0:1}

echo $SERVICE_PORT
```

#### SSL Disabled

##### Produce Messages

```bash
kafka-console-producer.sh --broker-list $SERVICE_IP:$SERVICE_PORT --topic my-topic
```

##### Consume Messages

```bash
kafka-console-consumer.sh --bootstrap-server $SERVICE_IP:$SERVICE_PORT --topic my-topic --from-beginning
```

#### SSL Enabled

To try Kafka guarded by SSL we recommend to use [Kafkacat](https://github.com/edenhill/kafkacat).

> Want to use the java client instead please generate the proper truststore and keystore using the [official docs](https://kafka.apache.org/documentation/#security_ssl).

__MacOS__:

```bash
brew install kafkacat
```

__Ubuntu__:

```bash
apt-get update
apt-get install kafkacat
```

Extract secrets from the given Kubernetes Secret:

```bash
kubectl get secrets -n kafka kafka-operator-server-cert -o jsonpath="{['data']['\tls.crt']}" | base64 -D > client.crt.pem
kubectl get secrets -n kafka kafka-operator-server-cert -o jsonpath="{['data']['\tls.key']}" | base64 -D > client.key.pem
kubectl get secrets -n kafka kafka-operator-server-cert -o jsonpath="{['data']['\ca.crt']}" | base64 -D > ca.crt.pem

```


##### Produce Messages

```bash
kafkacat -b $SERVICE_IP:$SERVICE_PORT -P -X security.protocol=SSL \
-X ssl.key.location=client.key.pem \
-X ssl.certificate.location=client.crt.pem \
-X ssl.ca.location=ca.crt.pem \
-t my-topic
```

##### Consume Messages

```bash
kafkacat -b $SERVICE_IP:$SERVICE_PORT -C -X security.protocol=SSL \
-X ssl.key.location=client.key.pem \
-X ssl.certificate.location=client.crt.pem \
-X ssl.ca.location=ca.crt.pem \
-t my-topic
```
