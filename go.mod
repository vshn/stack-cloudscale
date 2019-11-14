module github.com/vshn/stack-cloudscale

go 1.13

require (
	github.com/aws/aws-sdk-go v1.25.23
	github.com/cloudscale-ch/cloudscale-go-sdk v1.0.0
	github.com/crossplaneio/crossplane v0.3.1-0.20191026093543-dfa760ae9cd2
	github.com/crossplaneio/crossplane-runtime v0.2.1
	github.com/crossplaneio/crossplane-tools v0.0.0-20191023215726-61fa1eff2a2e
	github.com/onsi/ginkgo v1.9.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.2
	sigs.k8s.io/controller-tools v0.2.1
)
