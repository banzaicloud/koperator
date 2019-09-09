## Securing Kafka With SSL

The `kafka-operator` makes securing your Kafka cluster with SSL simple.
You may provide your own certificates, or instruct the operator to create them for you
from your cluster configuration:

Below is an example listeners configuration for SSL:

```yaml
listenersConfig:
  externalListeners:
    - type: "ssl"
      name: "external"
      externalStartingPort: 19090
      containerPort: 29092
  internalListeners:
    - type: "ssl"
      name: "ssl"
      containerPort: 29092
      usedForInnerBrokerCommunication: true
  sslSecrets:
    tlsSecretName: "test-kafka-operator"
    jksPasswordName: "test-kafka-operator-pass"
    create: true
```

If `sslSecrets.create` is `false`, the operator will look for the secret at `sslSecrets.tlsSecretName` and expect these values:

| Key          | Value              |
|:------------:|:-------------------|
| `caCert`     | The CA certificate |
| `caKey`      | The CA private key |
| `clientCert` | A client certificate (this will be used by cruise control and the operator for kafka operations) |
| `clientKey`  | The private key for `clientCert` |
| `peerCert`   | The certificate for the kafka brokers |
| `peerKey`    | The private key for the kafka brokers |


## Using Kafka ACLs with SSL

If you choose not to enable ACLs for your kafka cluster, you may still use the `KafkaUser` resource to create new certificates for your applications.
You can leave the `topicGrants` out as they will not have any effect.

To enable ACL support for your kafka cluster, pass the following configurations along with your `brokerConfig`:

```
authorizer.class.name=kafka.security.auth.SimpleAclAuthorizer
allow.everyone.if.no.acl.found=false
```

The operator will ensure that cruise control and itself can still access the cluster, however, to create new clients
you will need to generate new certificates signed by the CA, and ensure ACLs on the topic.

The operator can automate this process for you using the `KafkaUser` CRD.

For example, to create a new producer for the topic `test-topic` against the KafkaCluster `kafka`, apply the following configuration:

```bash
cat << EOF | kubectl apply -n kafka -f -
apiVersion: banzaicloud.banzaicloud.io/v1alpha1
kind: KafkaUser
metadata:
  name: example-producer
  namespace: kafka
spec:
  clusterRef:
    name: kafka
  secretName: example-producer-secret
  topicGrants:
    - topicName: test-topic
      accessType: write
EOF
```

This will create a user and store its credentials in the secret `example-producer-secret`. The secret contains these fields:

| Key          | Value                |
|:------------:|:---------------------|
| `ca.crt`     | The CA certificate   |
| `tls.crt`    | The user certificate |
| `tls.key`    | The user private key |

You can then mount these secrets to your pod, or to write them to your local machine you could do the following:

```bash
kubectl get secret example-producer-secret -o jsonpath="{['data']['ca\.crt']}" | base64 -d > ca.crt
kubectl get secret example-producer-secret -o jsonpath="{['data']['tls\.crt']}" | base64 -d > tls.crt
kubectl get secret example-producer-secret -o jsonpath="{['data']['tls\.key']}" | base64 -d > tls.key
```

If you wanted to create a consumer for the topic you could then run this command:

```bash
cat << EOF | kubectl apply -n kafka -f -
apiVersion: banzaicloud.banzaicloud.io/v1alpha1
kind: KafkaUser
metadata:
  name: example-consumer
  namespace: kafka
spec:
  clusterRef:
    name: kafka
  secretName: example-consumer-secret
  includeJKS: true
  topicGrants:
    - topicName: test-topic
      accessType: read
EOF
```

The operator can also include a Java keystore format (JKS) with your user secret if you'd like.
You just need to add `includeJKS: true` to the `spec` like shown above, and then the user-secret will gain these additional fields:

| Key                     | Value                |
|:-----------------------:|:---------------------|
| `tls.jks`               | The java keystore containing both the user keys and the CA (use this for your keystore AND truststore) |
| `pass.txt`              | The password to decrypt the JKS (this will be randomly generated) |
| `client-ssl.properties` | An example properties file that can be used with a Java application or the `kafka-console-consumer` and `kafka-console-producer` |
