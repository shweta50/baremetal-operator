module github.com/platform9/pf9-addon-operator

go 1.13

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/go-logr/logr v0.2.1
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/platform9/pf9-qbert/sunpike/apiserver v0.0.0
	github.com/platform9/pf9-qbert/sunpike/conductor v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	google.golang.org/grpc v1.33.2
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v0.20.6
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/platform9/pf9-qbert/sunpike/apiserver => ../pf9-qbert/sunpike/apiserver
	github.com/platform9/pf9-qbert/sunpike/apiserver/generated => ../pf9-qbert/sunpike/apiserver/generated
	github.com/platform9/pf9-qbert/sunpike/conductor => ../pf9-qbert/sunpike/conductor
)
