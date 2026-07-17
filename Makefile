# VERSION defines the project version used to tag the manager image by default.
VERSION ?= 0.1.0

# Image URL to use all building/pushing image targets
IMG ?= docker.io/epamedp/tekton-pipeline-queue:$(VERSION)
# YEAR defines the year value used for substituting the YEAR placeholder in the boilerplate header.
YEAR ?= $(shell date +%Y)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	mkdir -p deploy-templates/crds
	cp config/crd/bases/*.yaml deploy-templates/crds/
	$(MAKE) api-docs

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt",year=$(YEAR) paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# e2e tests are Chainsaw-based (tests/e2e/chainsaw/) and follow the
# cd-pipeline-operator approach: `make start-kind && make e2e`. All cluster
# provisioning (Tekton Pipelines + the operator Helm chart) is done by
# install steps inside each Chainsaw scenario, driven by the standard KRCI
# e2e env contract below — the same contract the e2e-chainsaw Tekton task
# provides when running the suite in a vcluster on the core cluster.
# `make e2e` only builds the image, loads it into Kind, and runs Chainsaw.
START_KIND_CLUSTER ?= true
KIND_CLUSTER_NAME ?= tekton-pipeline-queue-test-e2e
KUBE_VERSION ?= 1.34
KIND_CONFIG ?= ./hack/kind-$(KUBE_VERSION).yaml

CONTAINER_REGISTRY_URL ?= docker.io
CONTAINER_REGISTRY_SPACE ?= epamedp
E2E_IMAGE_REPOSITORY ?= tekton-pipeline-queue
ifeq ($(origin E2E_IMAGE_TAG), undefined)
E2E_IMAGE_TAG := e2e-$(shell date +%s)
endif
E2E_IMG := $(CONTAINER_REGISTRY_URL)/$(CONTAINER_REGISTRY_SPACE)/$(E2E_IMAGE_REPOSITORY):$(E2E_IMAGE_TAG)

.PHONY: start-kind
start-kind: ## Start the e2e Kind cluster (set START_KIND_CLUSTER=false to skip)
ifeq (true,$(START_KIND_CLUSTER))
	$(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CONFIG)
endif

# LOAD_KIND_IMAGE=true builds the operator image and loads it into the local
# Kind cluster before running the suite. Set to false when the target cluster
# can pull the image from a registry (e.g. a vcluster on the core cluster,
# image pushed by CI) — then pass the matching E2E_IMAGE_* variables.
LOAD_KIND_IMAGE ?= true

.PHONY: e2e
e2e: build chainsaw ## Run the full Chainsaw e2e suite against the CURRENT kubeconfig context (cluster-agnostic; scenarios self-provision prerequisites)
ifeq (true,$(LOAD_KIND_IMAGE))
	$(CONTAINER_TOOL) build -t $(E2E_IMG) .
	$(KIND) load docker-image $(E2E_IMG) --name $(KIND_CLUSTER_NAME)
endif
	CONTAINER_REGISTRY_URL=$(CONTAINER_REGISTRY_URL) \
	CONTAINER_REGISTRY_SPACE=$(CONTAINER_REGISTRY_SPACE) \
	E2E_IMAGE_REPOSITORY=$(E2E_IMAGE_REPOSITORY) \
	E2E_IMAGE_TAG=$(E2E_IMAGE_TAG) \
	"$(CHAINSAW)" test --config tests/e2e/chainsaw/.chainsaw.yaml tests/e2e/chainsaw

.PHONY: delete-kind
delete-kind: ## Delete the e2e Kind cluster
	@$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

.PHONY: api-docs
api-docs: crdoc ## Generate CRD reference docs from deploy-templates/crds into docs/api.md.
	"$(CRDOC)" --resources deploy-templates/crds --output docs/api.md

.PHONY: helm-docs
helm-docs: helmdocs ## Generate deploy-templates/README.md from README.md.gotmpl.
	"$(HELMDOCS)"

.PHONY: validate-docs
validate-docs: api-docs helm-docs ## Fail if generated helm/api docs are out of date.
	@git diff -s --exit-code deploy-templates/README.md || (echo "Run 'make helm-docs' to address the issue." && git diff && exit 1)
	@git diff -s --exit-code docs/api.md || (echo "Run 'make api-docs' to address the issue." && git diff && exit 1)

.PHONY: changelog
changelog: git-chglog ## Generate CHANGELOG.md.
ifneq (${NEXT_RELEASE_TAG},)
	"$(GITCHGLOG)" --next-tag v${NEXT_RELEASE_TAG} -o CHANGELOG.md
else
	"$(GITCHGLOG)" -o CHANGELOG.md
endif

##@ Build

# The binary is a container artifact, so GOOS defaults to linux regardless of
# the host (use `make run` to execute locally). GOARCH is overridable so CI
# can produce both architectures for multi-arch image builds:
#   GOARCH=amd64 make build && GOARCH=arm64 make build
# The Dockerfile only copies dist/manager-${TARGETARCH}; it compiles nothing.
GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)

.PHONY: build
build: manifests generate fmt vet ## Build manager binary into dist/manager-$(GOARCH).
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o dist/manager-$(GOARCH) cmd/main.go

.PHONY: clean
clean:  ## clean up
	-rm -rf dist
	-rm -f cover.out

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: build ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# The Dockerfile copies prebuilt dist/manager-<arch> binaries, so every listed
# platform needs a matching `GOARCH=<arch> make build` prerequisite below.
PLATFORMS ?= linux/arm64,linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	GOARCH=amd64 $(MAKE) build
	GOARCH=arm64 $(MAKE) build
	- $(CONTAINER_TOOL) buildx create --name tekton-pipeline-queue-builder
	$(CONTAINER_TOOL) buildx use tekton-pipeline-queue-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} .
	- $(CONTAINER_TOOL) buildx rm tekton-pipeline-queue-builder

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
CRDOC ?= $(LOCALBIN)/crdoc
HELMDOCS ?= $(LOCALBIN)/helm-docs
GITCHGLOG ?= $(LOCALBIN)/git-chglog
CHAINSAW_VERSION ?= v0.2.15
CHAINSAW ?= $(LOCALBIN)/chainsaw-$(CHAINSAW_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.21.0
CRDOC_VERSION ?= v0.6.4
HELMDOCS_VERSION ?= v1.14.2
GITCHGLOG_VERSION ?= v0.15.4

#ENVTEST_VERSION is the controller-runtime version to use for setup-envtest, derived from go.mod
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v")

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.12.2
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	@test -f .custom-gcl.yml && { \
		echo "Building custom golangci-lint with plugins..." && \
		$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom && \
		mv -f $(LOCALBIN)/golangci-lint-custom $(GOLANGCI_LINT); \
	} || true

.PHONY: crdoc
crdoc: $(CRDOC) ## Download crdoc locally if necessary.
$(CRDOC): $(LOCALBIN)
	$(call go-install-tool,$(CRDOC),fybrik.io/crdoc,$(CRDOC_VERSION))

.PHONY: helmdocs
helmdocs: $(HELMDOCS) ## Download helm-docs locally if necessary.
$(HELMDOCS): $(LOCALBIN)
	$(call go-install-tool,$(HELMDOCS),github.com/norwoodj/helm-docs/cmd/helm-docs,$(HELMDOCS_VERSION))

.PHONY: git-chglog
git-chglog: $(GITCHGLOG) ## Download git-chglog locally if necessary.
$(GITCHGLOG): $(LOCALBIN)
	$(call go-install-tool,$(GITCHGLOG),github.com/git-chglog/git-chglog/cmd/git-chglog,$(GITCHGLOG_VERSION))

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	$(call go-install-tool,$(LOCALBIN)/chainsaw,github.com/kyverno/chainsaw,$(CHAINSAW_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
