module github.com/banzaicloud/koperator

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/Shopify/sarama v1.29.1
	github.com/banzaicloud/bank-vaults/pkg/sdk v0.7.0
	github.com/banzaicloud/istio-client-go v0.0.10
	github.com/banzaicloud/istio-operator/pkg/apis v0.10.5
	github.com/banzaicloud/k8s-objectmatcher v1.5.2
	github.com/banzaicloud/koperator/api v0.0.0
	github.com/banzaicloud/koperator/properties v0.1.0
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cncf/xds/go v0.0.0-20210805033703-aa0b78936158 // indirect
	github.com/envoyproxy/go-control-plane v0.9.9
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.1.5 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/hashicorp/vault v1.8.2
	github.com/hashicorp/vault/api v1.1.2-0.20210713235431-1fc8af4c041f
	github.com/hashicorp/vault/sdk v0.2.2-0.20210825150427-9b1f4d486f5d
	github.com/imdario/mergo v0.3.12
	github.com/influxdata/influxdb v1.9.3 // indirect
	github.com/jetstack/cert-manager v1.5.3
	github.com/klauspost/compress v1.13.5 // indirect
	github.com/lestrrat-go/backoff v1.0.1
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/pavel-v-chernykh/keystore-go/v4 v4.1.0
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/prometheus/common v0.30.0
	github.com/shirou/gopsutil v3.21.8+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/sys v0.0.0-20210902050250-f475640dd07b // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210831024726-fe130286e0e2 // indirect
	google.golang.org/protobuf v1.27.1
	gopkg.in/inf.v0 v0.9.1 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kube-openapi v0.0.0-20210817084001-7fbd8d59e5b8 // indirect
	k8s.io/utils v0.0.0-20210820185131-d34e5cb4466e // indirect
	sigs.k8s.io/controller-runtime v0.9.6
)

replace (
	github.com/banzaicloud/koperator/api => ./api
	github.com/banzaicloud/koperator/properties => ./properties
)
