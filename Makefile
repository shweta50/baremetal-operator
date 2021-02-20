CGO_ENABLED=0
GOOS=darwin
GOARCH=amd64
COMPILE_IMAGE=golang:mk
REPO_URL=github.com
ORG=platform9
REPO=pf9-addon-operator
COMPONENTS=agent
DOCKER=docker
DOCKER_PULL_CMD=${DOCKER} pull
DOCKER_PUSH_CMD=${DOCKER} push
DOCKER_RUN_CMD=${DOCKER} run --rm
DOCKER_BUILD_CMD=${DOCKER} build
DOCKER_RMI_CMD=${DOCKER} rmi
RM=rm
REGISTRY=docker.io
IMAGE_PROJECT=platform9
RELEASE=1.0.0

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
	for component in ${COMPONENTS}; do \
		echo "Compiling and building $${component} static binary"; \
		${DOCKER_RUN_CMD} -e CGO_ENABLED=${CGO_ENABLED} \
			-e GO111MODULE="on" \
			-e GOOS=${GOOS} \
			-e GOARCH=${GOARCH} \
			-v ${PWD}:/go/src/${REPO_URL}/${ORG}/${REPO}:Z \
			-v ${PWD}/bin:/go/bin:Z \
			-w /go/src/${REPO_URL}/${ORG}/${REPO} \
			${COMPILE_IMAGE} \
			sh -c \
            "go mod download && \
            go build -ldflags \"-extldflags \"-static\"\" -gcflags=\"all=-N -l\" \
			-o /go/bin/$${component} \
			${REPO_URL}/${ORG}/${REPO}/cmd/$${component}";\
	done

build: compile
	for component in ${COMPONENTS}; do \
		echo "Building docker image for $${component}"; \
		${DOCKER_BUILD_CMD} --network host -t ${REGISTRY}/${IMAGE_PROJECT}/$${component}:${RELEASE} \
			-f ${PWD}/tooling/$${component}.df \
			${PWD}; \
	done

release: build
	for component in ${COMPONENTS}; do \
		echo "Pushing docker image for $${component} to registy"; \
		${DOCKER_PUSH_CMD} ${REGISTRY}/${IMAGE_PROJECT}/$${component}:${RELEASE}; \
	done


# Cleanup build artifacts
clean:
	- for component in ${COMPONENTS}; do \
		  ${DOCKER_RUN_CMD} \
			-v ${PWD}/bin:/go/bin:Z \
			${COMPILE_IMAGE} ${RM} -f /go/bin/$${component} && \
			${DOCKER_RMI_CMD} ${REGISTRY}/${IMAGE_PROJECT}/$${component}:${RELEASE}; \
		done
	- rm -rf ${PWD}/bin


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
#ifeq (, $(shell which controller-gen))
#	@{ \
#	set -e ;\
#	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
#	cd $$CONTROLLER_GEN_TMP_DIR ;\
#	go mod init tmp ;\
#	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
#	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
#	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
#else
#CONTROLLER_GEN=$(shell which controller-gen)
#endif
