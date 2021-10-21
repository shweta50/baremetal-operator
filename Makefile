CGO_ENABLED=0
GOOS=linux
GOARCH=amd64
RELEASE=5.0.0-build
KUBERNETES_VERSION="v1.21.3"
KUBERNETES_GITHUB_RAW_BASEURL := https://raw.githubusercontent.com/kubernetes/kubernetes/${KUBERNETES_VERSION}
WGET_CMD := wget --progress=dot:giga



# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

.PHONY: all release clean
default: compile

all: compile build

# Compile and generate static binaries
compile: 
	echo "Compiling and building static binary"; \
	CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} \
	go build -o bin/manager cmd/agent/agent.go 

unit-tests: 
	echo "Running Unit tests"; \
	UNIT_TEST=true go test ./tests/...

build: compile
	echo "Building docker image"; \
    chmod +x bin/manager
	docker build -t platform9/pf9-addon-operator:${RELEASE} -f tooling/agent.df .


release: build
	echo "Pushing docker image to registy"; \
	docker push platform9/pf9-addon-operator:${RELEASE}

get-latest-addons:
	mkdir -p /tmp/coredns /tmp/metrics-server
	cd /tmp/coredns && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/dns/coredns/coredns.yaml.in
	cd /tmp/metrics-server && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/auth-delegator.yaml && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/auth-reader.yaml && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/metrics-apiservice.yaml && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/metrics-server-deployment.yaml && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/metrics-server-service.yaml && \
	${WGET_CMD} $(KUBERNETES_GITHUB_RAW_BASEURL)/cluster/addons/metrics-server/resource-reader.yaml
        

# Cleanup build artifacts
clean:
	rm -rf ${PWD}/bin


# Build manager binary
manager: generate fmt vet
	go build -o bin/manager cmd/agent/agent.go


# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/agent.pf9.io_addons.yaml tooling/manifests/pf9-addon-operator/pf9-addon-operator-crd.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
