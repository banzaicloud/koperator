module github.com/banzaicloud/kafka-operator

go 1.14

require (
	emperror.dev/errors v0.7.0
	github.com/Shopify/sarama v1.27.0
	github.com/banzaicloud/bank-vaults/pkg/sdk v0.3.0
	github.com/banzaicloud/istio-client-go v0.0.0-20200410173743-e1adddc949b3
	github.com/banzaicloud/istio-operator v0.0.0-20200407070503-3f7dc6953a7b
	github.com/banzaicloud/k8s-objectmatcher v1.4.1
	github.com/banzaicloud/kafka-operator/api v0.0.0
	github.com/envoyproxy/go-control-plane v0.9.4
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.3.5
	github.com/hashicorp/memberlist v0.2.0 // indirect
	github.com/hashicorp/vault v1.4.0
	github.com/hashicorp/vault/api v1.0.5-0.20200317185738-82f498082f02
	github.com/hashicorp/vault/sdk v0.1.14-0.20200406173424-43a93d4a43b1
	github.com/imdario/mergo v0.3.9
	github.com/influxdata/influxdb v1.7.6 // indirect
	github.com/jetstack/cert-manager v0.15.1
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lestrrat-go/backoff v1.0.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible
	github.com/prometheus/common v0.9.1
	go.opencensus.io v0.22.0 // indirect
	k8s.io/api v0.18.2
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
)

replace github.com/banzaicloud/kafka-operator/api => ./pkg/sdk
