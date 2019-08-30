module github.com/banzaicloud/kafka-operator

go 1.12

require (
	emperror.dev/emperror v0.21.3
	emperror.dev/errors v0.4.2
	github.com/Shopify/sarama v1.23.1
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190717042225-c3de453c63f4 // indirect
	github.com/banzaicloud/k8s-objectmatcher v1.0.0
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/envoyproxy/go-control-plane v0.8.6
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/gogo/googleapis v1.2.0 // indirect
	github.com/gogo/protobuf v1.2.2-0.20190730201129-28a6bbf47e48
	github.com/lestrrat-go/backoff v0.0.0-20190107202757-0bc2a4274cd0
	github.com/lyft/protoc-gen-validate v0.1.0 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/prometheus/common v0.0.0-20180801064454-c7de2306084e
	golang.org/x/net v0.0.0-20190501004415-9ce7a6920f09 // indirect
	golang.org/x/sys v0.0.0-20190429190828-d89cdac9e872 // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/grpc v1.23.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
	k8s.io/api v0.0.0-20190528154508-67ef80593b24
	k8s.io/apimachinery v0.0.0-20190528154326-e59c2fb0a8e5
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
