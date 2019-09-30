module github.com/banzaicloud/kafka-operator

go 1.12

require (
	emperror.dev/errors v0.4.2
	github.com/Shopify/sarama v1.23.1
	github.com/banzaicloud/k8s-objectmatcher v1.0.0
	github.com/envoyproxy/go-control-plane v0.8.6
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/gogo/googleapis v1.2.0 // indirect
	github.com/gogo/protobuf v1.2.2-0.20190730201129-28a6bbf47e48
	github.com/hashicorp/vault v1.2.3
	github.com/hashicorp/vault/api v1.0.5-0.20190909201928-35325e2c3262
	github.com/hashicorp/vault/sdk v0.1.14-0.20190909201848-e0fbf9b652e2
	github.com/imdario/mergo v0.3.7
	github.com/jetstack/cert-manager v0.9.1
	github.com/lestrrat-go/backoff v0.0.0-20190107202757-0bc2a4274cd0
	github.com/lyft/protoc-gen-validate v0.1.0 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible
	github.com/prometheus/common v0.2.0
	google.golang.org/grpc v1.23.0 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	k8s.io/api v0.0.0-20190718183219-b59d8169aab5
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v11.0.1-0.20190516230509-ae8359b20417+incompatible
	sigs.k8s.io/controller-runtime v0.2.0
)

replace (
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.6.7
	k8s.io/api => k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b

	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190409022649-727a075fdec8
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go => k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)
