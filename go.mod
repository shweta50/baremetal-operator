module github.com/platform9/pf9-addon-operator

go 1.13

require (
	github.com/fluxcd/pkg/untar v0.0.5
	github.com/go-logr/logr v0.2.1
	github.com/kudobuilder/kudo v0.17.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/kustomize/api v0.6.5
	sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06 // indirect
	sigs.k8s.io/yaml v1.2.0
)
