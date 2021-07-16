module github.com/banzaicloud/kafka-operator

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/Shopify/sarama v1.29.1
	github.com/banzaicloud/bank-vaults/pkg/sdk v0.7.0
	github.com/banzaicloud/istio-client-go v0.0.10
	github.com/banzaicloud/istio-operator/pkg/apis v0.0.0-20210715163935-19a792650a75
	github.com/banzaicloud/k8s-objectmatcher v1.5.1
	github.com/banzaicloud/kafka-operator/api v0.0.0
	github.com/banzaicloud/kafka-operator/properties v0.1.0
	github.com/cncf/xds/go v0.0.0-20210323124008-b88cc788a63e // indirect
	github.com/envoyproxy/go-control-plane v0.9.9
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.4.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.1.5 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-3 // indirect
	github.com/hashicorp/vault v1.7.3
	github.com/hashicorp/vault/api v1.1.1
	github.com/hashicorp/vault/sdk v0.2.1
	github.com/imdario/mergo v0.3.12
	github.com/influxdata/influxdb v1.9.3 // indirect
	github.com/jetstack/cert-manager v1.4.0
	github.com/klauspost/compress v1.12.3 // indirect
	github.com/lestrrat-go/backoff v1.0.1
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/prometheus/common v0.29.0
	github.com/shirou/gopsutil v3.21.6+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.6 // indirect
	go.opencensus.io v0.22.6 // indirect
	go.uber.org/zap v1.18.1
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914 // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	google.golang.org/genproto v0.0.0-20210716133855-ce7ef5c701ea // indirect
	google.golang.org/protobuf v1.27.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/kube-openapi v0.0.0-20210527164424-3c818078ee3d // indirect
	k8s.io/utils v0.0.0-20210709001253-0e1f9d693477 // indirect
	sigs.k8s.io/controller-runtime v0.9.2
)

replace (
	github.com/banzaicloud/kafka-operator/api => ./pkg/sdk
	github.com/banzaicloud/kafka-operator/properties => ./properties
)
