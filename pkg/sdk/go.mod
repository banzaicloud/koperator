module github.com/banzaicloud/kafka-operator/pkg/sdk

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/banzaicloud/istio-client-go v0.0.9
	github.com/google/go-cmp v0.4.1 // indirect
	github.com/imdario/mergo v0.3.11
	github.com/jetstack/cert-manager v0.15.2
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	sigs.k8s.io/controller-runtime v0.6.3
)
