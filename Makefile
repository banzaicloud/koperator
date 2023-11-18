SHELL = /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(PROJECT_DIR)/bin
BOILERPLATE_DIR := $(PROJECT_DIR)/hack/boilerplate

# Image URL to use all building/pushing image targets
TAG ?= $(shell git describe --tags --abbrev=0 --match 'v[0-9].*[0-9].*[0-9]' 2>/dev/null )
IMG ?= ghcr.io/banzaicloud/kafka-operator:$(TAG)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

RELEASE_TYPE ?= p
RELEASE_MSG ?= "koperator release"

REL_TAG = $(shell ./scripts/increment_version.sh -${RELEASE_TYPE} ${TAG})

GOLANGCI_VERSION = 1.55.2
LICENSEI_VERSION = 0.9.0
GOPROXY=https://proxy.golang.org

CONTROLLER_GEN_VERSION = v0.13.0
CONTROLLER_GEN = $(PWD)/bin/controller-gen

ENVTEST_K8S_VERSION = 1.27.x!

KUSTOMIZE_BASE = config/overlays/specific-manager-version

HELM_CRD_PATH = charts/kafka-operator/crds

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export PATH := $(PWD)/bin:$(PATH)

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^([a-zA-Z_0-9-]|\/)+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

all: test manager ## Run 'test' and 'manager' targets.

.PHONY: check
check: test lint ## Run tests and linters

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION} ## Symlink golangi-lint-<version> into versionless golangci-lint.
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint
bin/golangci-lint-${GOLANGCI_VERSION}: ## Download versioned golangci-lint.
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s -- -b ./bin v${GOLANGCI_VERSION}
	@mv bin/golangci-lint $@

.PHONY: lint
lint: bin/golangci-lint ## Run linter analysis.
	bin/golangci-lint run -c ./.golangci.yml --timeout=5m
	cd api && ../bin/golangci-lint run -c ../.golangci.yml --timeout=5m
	cd properties && ../bin/golangci-lint run -c ../.golangci.yml --timeout=5m
	cd tests/e2e && ../../bin/golangci-lint run -c ../../.golangci.yml --timeout=5m

.PHONY: lint-fix ## Run linter with automatic fixes.
lint-fix: bin/golangci-lint ## Run linter
	@bin/golangci-lint run -v --fix

bin/licensei: bin/licensei-${LICENSEI_VERSION} ## Symlink licensei-<version> into versionless licensei.
	@ln -sf licensei-${LICENSEI_VERSION} bin/licensei
bin/licensei-${LICENSEI_VERSION}: ## Download versioned licensei.
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/goph/licensei/master/install.sh | bash -s v${LICENSEI_VERSION}
	@mv bin/licensei $@

.PHONY: license-check
license-check: bin/licensei ## Run license check.
	bin/licensei check

.PHONY: license-cache
license-cache: bin/licensei ## Generate license cache.
	bin/licensei cache

install-kustomize: ## Install kustomize.
	@ if ! which bin/kustomize &>/dev/null; then\
		scripts/install_kustomize.sh;\
	fi

test: generate fmt vet manifests bin/setup-envtest ## Run unit and integration (non-e2e) tests.
	cd api && go test ./...
	KUBEBUILDER_ASSETS=$$($(BIN_DIR)/setup-envtest --print path --bin-dir $(BIN_DIR) use $(ENVTEST_K8S_VERSION)) \
	go test ./... \
		-coverprofile cover.out \
		-v \
		-failfast \
		-test.v \
		-test.paniconexit0 \
		-timeout 1h
	cd properties && go test -coverprofile cover.out -cover -failfast -v -covermode=count ./pkg/... ./internal/...

# Run e2e tests
test-e2e:
	 IMG_E2E=${IMG_E2E} go test github.com/banzaicloud/koperator/tests/e2e \
		-v \
		-timeout 20m \
		-tags e2e \
		--ginkgo.show-node-events \
		--ginkgo.trace \
		--ginkgo.v

manager: generate fmt vet ## Generate (kubebuilder) and build manager binary.
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests ## Run the generated manager against the configured Kubernetes cluster.
	go run ./main.go

# Install CRDs into a cluster by manually creating or replacing the CRD depending on whether is currently existing
# Apply is not applicable as the last-applied-configuration annotation would exceed the size limit enforced by the api server
install: manifests ## Install generated CRDs into the configured Kubernetes cluster.
	kubectl create -f config/base/crds || kubectl replace -f config/base/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install-kustomize install ## Deploy controller into the configured Kubernetes cluster.
	# creates the kafka namespace
	bin/kustomize build config | kubectl apply -f -
	./scripts/image_patch.sh "${KUSTOMIZE_BASE}/manager_image_patch.yaml" ${IMG}
	bin/kustomize build $(KUSTOMIZE_BASE) | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: bin/controller-gen ## Generate (Kubebuilder) manifests e.g. CRD, RBAC etc.
	cd api && $(CONTROLLER_GEN) $(CRD_OPTIONS) webhook paths="./..." output:crd:artifacts:config=../config/base/crds output:webhook:artifacts:config=../config/base/webhook
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./controllers/..." output:rbac:artifacts:config=./config/base/rbac
	## Regenerate CRDs for the helm chart
	cp config/base/crds/kafka.banzaicloud.io_cruisecontroloperations.yaml $(HELM_CRD_PATH)/cruisecontroloperations.yaml
	cp config/base/crds/kafka.banzaicloud.io_kafkaclusters.yaml $(HELM_CRD_PATH)/kafkaclusters.yaml
	cp config/base/crds/kafka.banzaicloud.io_kafkatopics.yaml $(HELM_CRD_PATH)/kafkatopics.yaml
	cp config/base/crds/kafka.banzaicloud.io_kafkausers.yaml $(HELM_CRD_PATH)/kafkausers.yaml

fmt: ## Run go fmt against code.
	go fmt ./...
	cd api && go fmt ./...
	cd properties && go fmt ./...

vet: ## Run go vet against code.
	go vet ./...
	cd api && go fmt ./...
	cd properties && go vet ./...

generate: bin/controller-gen gen-license-header ## Generate source code for APIs, Mocks, etc.
	cd api && $(CONTROLLER_GEN) object:headerFile=$(BOILERPLATE_DIR)/header.go.generated.txt paths="./..."

docker-build: ## Build the operator docker image.
	docker build . -t ${IMG}

docker-push: ## Push the operator docker image.
	docker push ${IMG}

bin/controller-gen: bin/controller-gen-$(CONTROLLER_GEN_VERSION) ## Symlink controller-gen-<version> into versionless controller-gen.
	@ln -sf controller-gen-$(CONTROLLER_GEN_VERSION) bin/controller-gen

bin/controller-gen-$(CONTROLLER_GEN_VERSION): ## Download versioned controller-gen.
	GOBIN=$(PWD)/bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	mv bin/controller-gen bin/controller-gen-$(CONTROLLER_GEN_VERSION)

# find or download setup-envtest

# https://github.com/kubernetes-sigs/controller-runtime/commits/main/tools/setup-envtest
SETUP_ENVTEST_VERSION := d4f1e822ca11e9ff149bf2d9b5285f375334eba5

bin/setup-envtest: $(BIN_DIR)/setup-envtest-$(SETUP_ENVTEST_VERSION) ## Symlink setup-envtest-<version> into versionless setup-envtest.
	@ln -sf setup-envtest-$(SETUP_ENVTEST_VERSION) $(BIN_DIR)/setup-envtest

$(BIN_DIR)/setup-envtest-$(SETUP_ENVTEST_VERSION): ## Download versioned setup-envtest.
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)
	@mv $(BIN_DIR)/setup-envtest $(BIN_DIR)/setup-envtest-$(SETUP_ENVTEST_VERSION)

check-release: ## Release confirmation.
	@echo "A new tag (${REL_TAG}) will be pushed to Github, and a new Docker image will be released. Are you sure? [y/N] " && read ans && [ $${ans:-N} == y ]

release: check-release ## Tag and push a release.
	git tag -a ${REL_TAG} -m ${RELEASE_MSG}
	git push origin ${REL_TAG}

update-go-deps: ## Update Go modules dependencies.
	for dir in api properties . test/e2e; do \
		( \
		echo "Updating $$dir deps"; \
		cd $$dir; \
		go mod tidy; \
		for m in $$(go list -mod=readonly -m -f '{{ if and (not .Replace) (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
			go get -u $$m; \
		done; \
		go mod tidy \
		) \
	done

ADDLICENSE_VERSION := 1.1.1

bin/addlicense: $(BIN_DIR)/addlicense-$(ADDLICENSE_VERSION) ## Symlink addlicense-<version> into versionless addlicense.
	@ln -sf addlicense-$(ADDLICENSE_VERSION) $(BIN_DIR)/addlicense

$(BIN_DIR)/addlicense-$(ADDLICENSE_VERSION): ## Download versioned addlicense.
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) go install github.com/google/addlicense@v$(ADDLICENSE_VERSION)
	@mv $(BIN_DIR)/addlicense $(BIN_DIR)/addlicense-$(ADDLICENSE_VERSION)

ADDLICENSE_SOURCE_DIRS := api controllers internal pkg properties scripts tests/e2e
ADDLICENSE_OPTS_IGNORE := -ignore '**/*.yml' -ignore '**/*.yaml' -ignore '**/*.xml'

.PHONY: license-header-check
license-header-check: gen-license-header bin/addlicense ## Find missing license header in source code files.
	bin/addlicense \
		-check \
		-f $(BOILERPLATE_DIR)/header.generated.txt \
		$(ADDLICENSE_OPTS_IGNORE) \
		$(ADDLICENSE_SOURCE_DIRS)

.PHONY: license-header-fix
license-header-fix: gen-license-header bin/addlicense ## Fix missing license header in source code files.
	bin/addlicense \
		-f $(BOILERPLATE_DIR)/header.generated.txt \
		$(ADDLICENSE_OPTS_IGNORE) \
		$(ADDLICENSE_SOURCE_DIRS)

GOTEMPLATE_VERSION := 3.7.5

bin/gotemplate: $(BIN_DIR)/gotemplate-$(GOTEMPLATE_VERSION) ## Symlink gotemplate-<version> into versionless gotemplate.
	@ln -sf gotemplate-$(GOTEMPLATE_VERSION) $(BIN_DIR)/gotemplate

$(BIN_DIR)/gotemplate-$(GOTEMPLATE_VERSION): ## Download versioned gotemplate.
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) go install github.com/coveooss/gotemplate/v3@v$(GOTEMPLATE_VERSION)
	@mv $(BIN_DIR)/gotemplate $(BIN_DIR)/gotemplate-$(GOTEMPLATE_VERSION)

.PHONY: gen-license-header
gen-license-header: bin/gotemplate ## Generate license header used in source code files.
	GOTEMPLATE_NO_STDIN=true \
	$(BIN_DIR)/gotemplate run \
		--follow-symlinks \
		--import="$(BOILERPLATE_DIR)/vars.yml" \
		--source="$(BOILERPLATE_DIR)"


MOCKGEN_VERSION := 0.2.0

bin/mockgen: $(BIN_DIR)/mockgen-$(MOCKGEN_VERSION) ## Symlink mockgen-<version> into versionless mockgen.
	@ln -sf mockgen-$(MOCKGEN_VERSION) $(BIN_DIR)/mockgen

$(BIN_DIR)/mockgen-$(MOCKGEN_VERSION): ## Download versioned mockgen.
	@mkdir -p $(BIN_DIR)
	@GOBIN=$(BIN_DIR) go install go.uber.org/mock/mockgen@v$(MOCKGEN_VERSION)
	@mv $(BIN_DIR)/mockgen $(BIN_DIR)/mockgen-$(MOCKGEN_VERSION)

.PHONY: mock-generate
mock-generate: bin/mockgen ## Generate mocks for specified interfaces.
	$(BIN_DIR)/mockgen \
		-copyright_file $(BOILERPLATE_DIR)/header.generated.txt \
		-package mocks \
		-source pkg/scale/types.go \
		-destination controllers/tests/mocks/scale.go
	$(BIN_DIR)/mockgen \
	    -copyright_file $(BOILERPLATE_DIR)/header.generated.txt \
	    -package mocks \
		-destination pkg/resources/kafka/mocks/Client.go \
		sigs.k8s.io/controller-runtime/pkg/client Client
	$(BIN_DIR)/mockgen \
		-copyright_file $(BOILERPLATE_DIR)/header.generated.txt \
		-package mocks \
		-source pkg/kafkaclient/client.go \
		-destination pkg/resources/kafka/mocks/KafkaClient.go
