# Koperator chart

The [Koperator](https://github.com/banzaicloud/koperator) is a Kubernetes operator to deploy and manage [Apache Kafka](https://kafka.apache.org) resources for a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.15.0+

## Installing the chart

Before installing the chart, you must first install the Koperator CustomResourceDefinition resources.
This is performed in a separate step to allow you to easily uninstall and reinstall Koperator without deleting your installed custom resources.

```
kubectl create --validate=false -f https://github.com/banzaicloud/koperator/releases/download/v0.25.1/kafka-operator.crds.yaml
```

To install the chart:

```
$ helm repo add banzaicloud-stable https://kubernetes-charts.banzaicloud.com
$ helm install kafka-operator --create-namespace --namespace=kafka banzaicloud-stable/kafka-operator
```

To install the operator using an already installed cert-manager
```bash
$ helm install kafka-operator --set certManager.namespace=<your cert manager namespace> --namespace=kafka  --create-namespace banzaicloud-stable/kafka-operator
```

## Upgrading the chart

To upgrade the chart since the helm 3 limitation you have to set a value as well to keep your CRDs.
If this value is not set your CRDs might be deleted.

```bash
helm upgrade kafka-operator --set --namespace=kafka banzaicloud-stable/kafka-operator
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
`operator.image.repository` | Operator container image repository | `ghcr.io/banzaicloud/kafka-operator`
`operator.image.tag` | Operator container image tag | `v0.26.0-dev.0`
`operator.image.pullPolicy` | Operator container image pull policy | `IfNotPresent`
`operator.serviceAccount.name` | ServiceAccount used by the operator pod | `kafka-operator`
`operator.serviceAccount.create` | If true, create the `operator.serviceAccount.name` service account | `true`
`operator.resources` | CPU/Memory resource requests/limits (YAML) | Memory: `128Mi/256Mi`, CPU: `100m/200m`
`operator.namespaces` | List of namespaces where Operator watches for custom resources.<br><br>**Note** that the operator still requires to read the cluster-scoped `Node` labels to configure `rack awareness`. Make sure the operator ServiceAccount is granted `get` permissions on this `Node` resource when using limited RBACs.| `""` i.e. all namespaces
`operator.annotations` | Operator pod annotations can be set | `{}`
`prometheusMetrics.enabled` | If true, use direct access for Prometheus metrics | `false`
`prometheusMetrics.authProxy.enabled` | If true, use auth proxy for Prometheus metrics | `true`
`prometheusMetrics.authProxy.serviceAccount.create` | If true, create the service account (see `prometheusMetrics.authProxy.serviceAccount.name`) used by prometheus auth proxy | `true`
`prometheusMetrics.authProxy.serviceAccount.name` | ServiceAccount used by prometheus auth proxy | `kafka-operator-authproxy`
`prometheusMetrics.authProxy.image.repository` | Auth proxy container image repository | `gcr.io/kubebuilder/kube-rbac-proxy`
`prometheusMetrics.authProxy.image.tag` | Auth proxy container image tag | `v0.15.0`
`prometheusMetrics.authProxy.image.pullPolicy` | Auth proxy container image pull policy | `IfNotPresent`
`rbac.enabled` | Create rbac service account and roles | `true`
`imagePullSecrets` | Image pull secrets can be set | `[]`
`replicaCount` | Operator replica count can be set | `1`
`alertManager.enable` | AlertManager can be enabled | `true`
`alertManager.permissivePeerAuthentication.create` | Permissive PeerAuthentication (Istio resource) for AlertManager can be created | `true`
`nodeSelector` | Operator pod node selector can be set | `{}`
`tolerations` | Operator pod tolerations can be set | `[]`
`affinity` | Operator pod affinity can be set | `{}`
`nameOverride` | Release name can be overwritten | `""`
`fullnameOverride` | Release full name can be overwritten | `""`
`certManager.namespace` | Operator will look for the cert manager in this namespace | `cert-manager`
`certManager.enabled` | Operator will integrate with the cert manager | `false`
`webhook.enabled` | Operator will activate the admission webhooks for custom resources | `true`
`webhook.certs.generate` | Helm chart will generate cert for the webhook | `true`
`webhook.certs.secret` | Helm chart will use the secret name applied here for the cert | `kafka-operator-serving-cert`
`additionalEnv` | Additional Environment Variables | `[]`
`additionalSidecars` | Additional Sidecars Configuration | `[]`
`additionalVolumes` | Additional volumes required for sidecars | `[]`
