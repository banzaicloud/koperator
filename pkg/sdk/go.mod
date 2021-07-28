module github.com/banzaicloud/kafka-operator/pkg/sdk

go 1.16

require (
	emperror.dev/errors v0.8.0
	github.com/banzaicloud/istio-client-go v0.0.10
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/jetstack/cert-manager v0.15.2
	golang.org/x/net v0.0.0-20210726213435-c6fcb2dbf985 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	sigs.k8s.io/controller-runtime v0.9.3
)
