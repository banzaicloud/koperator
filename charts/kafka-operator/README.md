# Kafka-operator chart

[Kafka-operator](https://github.com/banzaicloud/kafka-operator) is a Kubernetes operator to deploy and manage [Kafka](https://kafka.apache.org) resources for a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.13.0+

## Installing the chart

To install the chart:

```
$ helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com
$ helm install --name=kafka-operator --namespace=kafka banzaicloud-stable/kafka-operator
```

To install the operator using an already installed cert-manager
```bash
$ helm install --name=kafka-operator --set certManager.namespace=<your cert manager namespace> --namespace=kafka banzaicloud-stable/kafka-operator
```

## Uninstalling the Chart

To uninstall/delete the `kafka-operator` release:

```
$ helm delete --purge kafka-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the Banzaicloud Kafka Operator chart and their default values.

Parameter | Description | Default
--------- | ----------- | -------
`operator.image.repository` | Operator container image repository | `banzaicloud/kafka-operator`
`operator.image.tag` | Operator container image tag | `0.6.0`
`operator.image.pullPolicy` | Operator container image pull policy | `IfNotPresent`
`operator.resources` | CPU/Memory resource requests/limits (YAML) | Memory: `128Mi/256Mi`, CPU: `100m/200m`
`operator.namespaces` | List of namespaces where Operator watches for custom resources. **Note** that is not compatible with `rackAwareness` setting in the `KafkaCluster` resource as that requires read access for _cluster-scoped_ `Node` labels. | `""` i.e. all namespaces
`prometheusMetrics.enabled` | If true, use direct access for Prometheus metrics | `false`
`prometheusMetrics.authProxy.enabled` | If true, use auth proxy for Prometheus metrics | `true`
`prometheusMetrics.authProxy.image.repository` | Auth proxy container image repository | `gcr.io/kubebuilder/kube-rbac-proxy`
`prometheusMetrics.authProxy.image.tag` | Auth proxy container image tag | `v0.4.0`
`prometheusMetrics.authProxy.image.pullPolicy` | Auth proxy container image pull policy | `IfNotPresent`
`rbac.enabled` | Create rbac service account and roles | `true`
`imagePullSecrets` | Image pull secrets can be set | `[]`
`replicaCount` | Operator replica count can be set | `1`
`alertManager.enable` | AlertManager can be enabled | `true`
`nodeSelector` | Operator pod node selector can be set | `{}`
`tolerations` | Operator pod tolerations can be set | `[]`
`affinity` | Operator pod affinity can be set | `{}`
`nameOverride` | Release name can be overwritten | `""`
`fullnameOverride` | Release full name can be overwritten | `""`
`certManager.namespace` | Operator will look for the cert manager in this namespace | `cert-manager`
`webhook.enabled` | Operator will activate the admission webhooks for custom resources | `true`
`webhook.certs.generate` | Helm chart will generate cert for the webhook | `true`
`webhook.certs.secret` | Helm chart will use the secret name applied here for the cert | `kafka-operator-serving-cert`
