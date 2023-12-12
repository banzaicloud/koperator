module github.com/banzaicloud/koperator/properties

go 1.21

require (
	emperror.dev/errors v0.8.1
	github.com/onsi/gomega v1.30.0
)

require (
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// remove once https://github.com/cert-manager/cert-manager/issues/5953 is fixed
replace github.com/Venafi/vcert/v4 => github.com/jetstack/vcert/v4 v4.9.6-0.20230127103832-3aa3dfd6613d
