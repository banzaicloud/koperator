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
apiVersion: kafka.banzaicloud.io/v1alpha1
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
apiVersion: kafka.banzaicloud.io/v1alpha1
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

## Using different secret/PKI backends

The operator supports using a back-end other than `cert-manager` for the PKI and user secrets.
For now there is just an additional option of using `vault`.
An easy way to get up and running quickly with `vault` on your Kubernetes cluster is to use [`bank-vaults`](https://github.com/banzaicloud/bank-vaults).
To set up `bank-vaults`, a `vault` instance, and the `vault-secrets-webhook`, you can run the following:

```bash
git clone https://github.com/banzaicloud/bank-vaults
cd bank-vaults

# setup the operator and a vault instance
kubectl apply -f operator/deploy/rbac.yaml
kubectl apply -f operator/deploy/operator-rbac.yaml
kubectl apply -f operator/deploy/operator.yaml
kubectl apply -f operator/deploy/cr.yaml

# install the pod injector webhook (optional)
helm install --namespace vault-infra --name vault-secrets-webhook banzaicloud-stable/vault-secrets-webhook
```

With a vault instance in the cluster, you can deploy the operator with vault credentials.
First create a secret with the vault token and CA certificate by running:

```bash
# These values match the manifests applied above, they may be different for you
VAULT_TOKEN=$(kubectl get secrets vault-unseal-keys -o jsonpath={.data.vault-root} | base64 --decode)
VAULT_CACERT=$(kubectl get secret vault-tls -o jsonpath="{.data.ca\.crt}" | base64 --decode)

# create the kafka namespace if you haven't already
kubectl create ns kafka

# Create a Kubernetes secret with the token and CA cert
kubectl -n kafka create secret generic vault-keys --from-literal=vault.token=${VAULT_TOKEN} --from-literal=ca.crt="${VAULT_CACERT}"
```

Then, if using the `kafka-operator` helm chart:

```bash
helm install \
  --name kafka-operator \
  --namespace kafka \
  --set operator.vaultAddress=https://vault.default.svc.cluster.local:8200 \
  --set operator.vaultSecret=vault-keys \
  banzaicloud-stable/kafka-operator
```

You will now be able to specify the `vault` back-end when using the managed PKI. Your `sslSecrets` would look like this:

```yaml
sslSecrets:
  tlsSecretName: "test-kafka-operator"
  jksPasswordName: "test-kafka-operator-pass"
  create: true
  pkiBackend: vault
```

When a cluster is using the `vault` back-end the `KafkaUser` CRs will store their secrets in `vault` instead of Kubernetes secrets.
For example, if you installed the `vault-secrets-webhook` above, you could create a `KafkaUser` and ingest the keys like so:

```yaml
# A KafkaUser with permission to read from 'test-topic'
apiVersion: kafka.banzaicloud.io/v1alpha1
kind: KafkaUser
metadata:
  name: test-kafka-consumer
spec:
  clusterRef:
    name: kafka
    namespace: kafka
  secretName: test-kafka-consumer
  topicGrants:
    - topicName: test-topic
      accessType: read
---
# A pod containing a consumer using the above credentials
apiVersion: v1
kind: Pod
metadata:
  name: kafka-test-pod
  annotations:
    # annotations for the vault-secrets-webhook
    vault.security.banzaicloud.io/vault-addr: "https://vault:8200"
    vault.security.banzaicloud.io/vault-tls-secret: "vault-tls"
spec:

  containers:

    # Container reading from topic with the consumer credentials
    - name: consumer
      image: banzaicloud/kafka-test:latest
      env:
        - name: KAFKA_MODE
          value: consumer
        - name: KAFKA_TLS_CERT
          value: "vault:secret/data/test-kafka-consumer#tls.crt"
        - name: KAFKA_TLS_KEY
          value: "vault:secret/data/test-kafka-consumer#tls.key"
        - name: KAFKA_TLS_CA
          value: "vault:secret/data/test-kafka-consumer#ca.crt"
```

When no `kv` mount is supplied for `secretName` like in the user above, the operator will assume the default `kv` mount at `secret/`.
You can pass the `secretName` as a full `vault` path to specify a different secrets mount to store your user certificates.
