
# Image URL to use all building/pushing image targets
DWCO_IMG ?= quay.io/che-incubator/devworkspace-che-operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq (,$(shell which kubectl))
ifeq (,$(shell which oc))
$(error oc or kubectl is required to proceed)
else
K8S_CLI := oc
endif
else
K8S_CLI := kubectl
endif

ifeq ($(shell $(K8S_CLI) api-resources --api-group='route.openshift.io'  2>&1 | grep -o routes),routes)
PLATFORM := openshift
else
PLATFORM := kubernetes
endif

all: manager

### test: Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

### manager: Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

### run: Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

debug: generate fmt vet manifests
	dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go --
	
### install: Install CRDs into a cluster
install: manifests
	kustomize build deploy/templates/crd | $(K8S_CLI) apply -f -

### uninstall: Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build deploy/templates/crd | $(K8S_CLI) delete -f -

### deploy: Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: generate_deployment
	$(K8S_CLI) apply -f deploy/deployment/$(PLATFORM)/combined.yaml

### generate_deployment: generates the deployment files in deploy/deployment
generate_deployment: manifests
	deploy/generate-deployment.sh --split-yaml

### manifests: Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=deploy/templates/crd/bases

### fmt: Run go fmt against code
fmt:
	go fmt ./...

### vet: Run go vet against code
vet:
	go vet ./...

### generate: Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

### docker-build: Build the docker image
docker-build: test
	docker build . -t ${DWCO_IMG} -f build/dockerfiles/Dockerfile

### docker-push: Push the docker image
docker-push:
	docker push ${DWCO_IMG}

### controller-gen: find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: help
### help: print this message
help: Makefile
	@echo 'Available rules:'
	@sed -n 's/^### /    /p' $< | awk 'BEGIN { FS=":" } { printf "%-30s -%s\n", $$1, $$2 }'
	@echo ''
	@echo 'Supported environment variables:'
	@echo '    DWCO_IMG                    - Image used for controller'
	@echo '    DWCO_NAMESPACE              - Namespace to use for deploying controller'
	@choe '    DWCO_PULL_POLICY            - Pull policy of the operator image in the operator deployment'
