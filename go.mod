module github.com/banzaicloud/kafka-operator

go 1.14

require (
	emperror.dev/errors v0.7.0
	github.com/Shopify/sarama v1.27.1
	github.com/banzaicloud/bank-vaults/pkg/sdk v0.3.1
	github.com/banzaicloud/istio-client-go v0.0.9
	github.com/banzaicloud/istio-operator v0.0.0-20200407070503-3f7dc6953a7b
	github.com/banzaicloud/k8s-objectmatcher v1.4.1
	github.com/banzaicloud/kafka-operator/api v0.0.0
	github.com/envoyproxy/go-control-plane v0.9.7
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.2
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/hashicorp/go-hclog v0.12.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.7 // indirect
	github.com/hashicorp/memberlist v0.2.0 // indirect
	github.com/hashicorp/vault v1.4.0
	github.com/hashicorp/vault/api v1.0.5-0.20200317185738-82f498082f02
	github.com/hashicorp/vault/sdk v0.1.14-0.20200406173424-43a93d4a43b1
	github.com/imdario/mergo v0.3.11
	github.com/influxdata/influxdb v1.7.6 // indirect
	github.com/jetstack/cert-manager v0.15.2
	github.com/klauspost/compress v1.11.1 // indirect
	github.com/lestrrat-go/backoff v1.0.0
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.4.1 // indirect
	github.com/prometheus/common v0.9.1
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/viper v1.7.1 // indirect
	go.uber.org/atomic v1.5.1 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/grpc v1.27.1 // indirect
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/controller-runtime v0.6.3
)

replace github.com/banzaicloud/kafka-operator/api => ./pkg/sdk
