SHELL := /bin/bash

# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

INGRESS_VERSION=4.9.1

KIND_CLUSTER ?= aergia-controller
KIND_NETWORK ?= aergia-controller

TIMEOUT = 30m

KIND_VERSION = v0.25.0
KUBECTL_VERSION := v1.31.0
HELM_VERSION := v3.16.1
GOJQ_VERSION = v0.12.16
KUSTOMIZE_VERSION := v5.4.3

HELM = $(realpath ./local-dev/helm)
KUBECTL = $(realpath ./local-dev/kubectl)
JQ = $(realpath ./local-dev/jq)
KIND = $(realpath ./local-dev/kind)
KUSTOMIZE = $(realpath ./local-dev/kustomize)

ARCH := $(shell uname | tr '[:upper:]' '[:lower:]')

.PHONY: local-dev/kind
local-dev/kind:
ifeq ($(KIND_VERSION), $(shell kind version 2>/dev/null | sed -nE 's/kind (v[0-9.]+).*/\1/p'))
	$(info linking local kind version $(KIND_VERSION))
	ln -sf $(shell command -v kind) ./local-dev/kind
else
ifneq ($(KIND_VERSION), $(shell ./local-dev/kind version 2>/dev/null | sed -nE 's/kind (v[0-9.]+).*/\1/p'))
	$(info downloading kind version $(KIND_VERSION) for $(ARCH))
	mkdir -p local-dev
	rm local-dev/kind || true
	curl -sSLo local-dev/kind https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(ARCH)-amd64
	chmod a+x local-dev/kind
endif
endif

.PHONY: local-dev/kustomize
local-dev/kustomize:
ifeq ($(KUSTOMIZE_VERSION), $(shell kustomize version 2>/dev/null | sed -nE 's/(v[0-9.]+).*/\1/p'))
	$(info linking local kustomize version $(KUSTOMIZE_VERSION))
	ln -sf $(shell command -v kustomize) ./local-dev/kustomize
else
ifneq ($(KUSTOMIZE_VERSION), $(shell ./local-dev/kustomize version 2>/dev/null | sed -nE 's/(v[0-9.]+).*/\1/p'))
	$(info downloading kustomize version $(KUSTOMIZE_VERSION) for $(ARCH))
	rm local-dev/kustomize || true
	curl -sSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_$(ARCH)_amd64.tar.gz | tar -xzC local-dev
	chmod a+x local-dev/kustomize
endif
endif

.PHONY: local-dev/helm
local-dev/helm:
ifeq ($(HELM_VERSION), $(shell helm version --short --client 2>/dev/null | sed -nE 's/(v[0-9.]+).*/\1/p'))
	$(info linking local helm version $(HELM_VERSION))
	ln -sf $(shell command -v helm) ./local-dev/helm
else
ifneq ($(HELM_VERSION), $(shell ./local-dev/helm version --short --client 2>/dev/null | sed -nE 's/(v[0-9.]+).*/\1/p'))
	$(info downloading helm version $(HELM_VERSION) for $(ARCH))
	rm local-dev/helm || true
	curl -sSL https://get.helm.sh/helm-$(HELM_VERSION)-$(ARCH)-amd64.tar.gz | tar -xzC local-dev --strip-components=1 $(ARCH)-amd64/helm
	chmod a+x local-dev/helm
endif
endif

.PHONY: local-dev/jq
local-dev/jq:
ifeq ($(GOJQ_VERSION), $(shell gojq -v 2>/dev/null | sed -nE 's/gojq ([0-9.]+).*/v\1/p'))
	$(info linking local gojq version $(GOJQ_VERSION))
	ln -sf $(shell command -v gojq) ./local-dev/jq
else
ifneq ($(GOJQ_VERSION), $(shell ./local-dev/jq -v 2>/dev/null | sed -nE 's/gojq ([0-9.]+).*/v\1/p'))
	$(info downloading gojq version $(GOJQ_VERSION) for $(ARCH))
	rm local-dev/jq || true
ifeq ($(ARCH), darwin)
	TMPDIR=$$(mktemp -d) \
		&& curl -sSL https://github.com/itchyny/gojq/releases/download/$(GOJQ_VERSION)/gojq_$(GOJQ_VERSION)_$(ARCH)_arm64.zip -o $$TMPDIR/gojq.zip \
		&& (cd $$TMPDIR && unzip gojq.zip) && cp $$TMPDIR/gojq_$(GOJQ_VERSION)_$(ARCH)_arm64/gojq ./local-dev/jq && rm -rf $$TMPDIR
else
	curl -sSL https://github.com/itchyny/gojq/releases/download/$(GOJQ_VERSION)/gojq_$(GOJQ_VERSION)_$(ARCH)_amd64.tar.gz | tar -xzC local-dev --strip-components=1 gojq_$(GOJQ_VERSION)_$(ARCH)_amd64/gojq
	mv ./local-dev/{go,}jq
endif
	chmod a+x local-dev/jq
endif
endif

.PHONY: local-dev/kubectl
local-dev/kubectl:
ifeq ($(KUBECTL_VERSION), $(shell kubectl version --client 2>/dev/null | grep Client | sed -E 's/Client Version: (v[0-9.]+).*/\1/'))
	$(info linking local kubectl version $(KUBECTL_VERSION))
	ln -sf $(shell command -v kubectl) ./local-dev/kubectl
else
ifneq ($(KUBECTL_VERSION), $(shell ./local-dev/kubectl version --client 2>/dev/null | grep Client | sed -E 's/Client Version: (v[0-9.]+).*/\1/'))
	$(info downloading kubectl version $(KUBECTL_VERSION) for $(ARCH))
	rm local-dev/kubectl || true
	curl -sSLo local-dev/kubectl https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/$(ARCH)/amd64/kubectl
	chmod a+x local-dev/kubectl
endif
endif

.PHONY: local-dev/tools
local-dev/tools: local-dev/kind local-dev/kustomize local-dev/kubectl local-dev/jq local-dev/helm 

.PHONY: helm/repos
helm/repos: local-dev/helm
	# install repo dependencies required by the charts
	$(HELM) repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
	$(HELM) repo add metallb https://metallb.github.io/metallb
	$(HELM) repo update

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
ENVTEST_VERSION ?= latest

all: manager

# Run tests
test: generate fmt vet manifests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager cmd/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run ./cmd/main.go --controller-namespace=${CONTROLLER_NAMESPACE}

# Install CRDs into a cluster
.PHONY: install
install: manifests
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
# Uninstall CRDs from a cluster
uninstall: manifests
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: preview
preview: manifests
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@export HARBOR_URL="https://registry.$$($(KUBECTL) -n ingress-nginx get services ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}' || echo 127.0.0.1).nip.io" && \
	$(KUSTOMIZE) build config/default

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
# this is only used locally for development or in the test suite
.PHONY: deploy
deploy: manifests
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@if $(KIND) get clusters | grep -q $(KIND_CLUSTER); then \
		$(KIND) export kubeconfig --name=$(KIND_CLUSTER) \
		&& export HARBOR_URL="https://registry.$$($(KUBECTL) -n ingress-nginx get services ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}').nip.io"; \
	fi && \
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: manifests
manifests: controller-gen local-dev/tools ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

controller-test:
	./controller-test.sh

clean:
	docker-compose down
	$(KIND) delete cluster --name ${KIND_NAME}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: create-kind-cluster
create-kind-cluster: local-dev/tools helm/repos
	docker network inspect $(KIND_NETWORK) >/dev/null || docker network create $(KIND_NETWORK) \
		&& LAGOON_KIND_CIDR_BLOCK=$$(docker network inspect $(KIND_NETWORK) | $(JQ) '. [0].IPAM.Config[0].Subnet' | tr -d '"') \
		&& export KIND_NODE_IP=$$(echo $${LAGOON_KIND_CIDR_BLOCK%???} | awk -F'.' '{print $$1,$$2,$$3,240}' OFS='.') \
 		&& envsubst < test-resources/test-suite.kind-config.yaml.tpl > test-resources/test-suite.kind-config.yaml \
 		&& envsubst < test/e2e/testdata/example-nginx.yaml.tpl > test/e2e/testdata/example-nginx.yaml \
		&& export KIND_EXPERIMENTAL_DOCKER_NETWORK=$(KIND_NETWORK) \
 		&& $(KIND) create cluster --wait=60s --name=$(KIND_CLUSTER) --config=test-resources/test-suite.kind-config.yaml

# Create a kind cluster locally and run the test e2e test suite against it
.PHONY: kind/test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up locally
kind/test-e2e: create-kind-cluster install-ingress kind/re-test-e2e
	
.PHONY: local-kind/test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up locally
kind/re-test-e2e:
	export KIND_PATH=$(KIND) && \
	export KUBECTL_PATH=$(KUBECTL) && \
	export KIND_CLUSTER=$(KIND_CLUSTER) && \
	LAGOON_KIND_CIDR_BLOCK=$$(docker network inspect $(KIND_NETWORK) | $(JQ) '. [0].IPAM.Config[0].Subnet' | tr -d '"') && \
	export KIND_NODE_IP=$$(echo $${LAGOON_KIND_CIDR_BLOCK%???} | awk -F'.' '{print $$1,$$2,$$3,240}' OFS='.') && \
	$(KIND) export kubeconfig --name=$(KIND_CLUSTER) && \
	$(MAKE) test-e2e

.PHONY: clean
kind/clean:
	$(KIND) delete cluster --name=$(KIND_CLUSTER) && docker network rm $(KIND_NETWORK)

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up inside github action.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: github/test-e2e
github/test-e2e: local-dev/tools install-ingress test-e2e

.PHONY: kind/set-kubeconfig
kind/set-kubeconfig:
	export KIND_CLUSTER=$(KIND_CLUSTER) && \
	$(KIND) export kubeconfig --name=$(KIND_CLUSTER)

.PHONY: kind/logs-aergia-controller
kind/logs-aergia-controller:
	export KIND_CLUSTER=$(KIND_CLUSTER) && \
	$(KIND) export kubeconfig --name=$(KIND_CLUSTER) && \
	$(KUBECTL) -n aergia-controller-system logs -f \
		$$($(KUBECTL) -n aergia-controller-system  get pod -l control-plane=controller-manager -o jsonpath="{.items[0].metadata.name}") \
		-c manager

.PHONY: install-metallb
install-metallb:
	LAGOON_KIND_CIDR_BLOCK=$$(docker network inspect $(KIND_NETWORK) | $(JQ) '. [0].IPAM.Config[0].Subnet' | tr -d '"') && \
	export LAGOON_KIND_NETWORK_RANGE=$$(echo $${LAGOON_KIND_CIDR_BLOCK%???} | awk -F'.' '{print $$1,$$2,$$3,240}' OFS='.')/29 && \
	$(HELM) upgrade \
		--install \
		--create-namespace \
		--namespace metallb-system  \
		--wait \
		--timeout $(TIMEOUT) \
		--version=v0.13.12 \
		metallb \
		metallb/metallb && \
	$$(envsubst < test-resources/test-suite.metallb-pool.yaml.tpl > test-resources/test-suite.metallb-pool.yaml) && \
	$(KUBECTL) apply -f test-resources/test-suite.metallb-pool.yaml

# installs ingress-nginx
.PHONY: install-ingress
install-ingress: install-metallb
	$(HELM) upgrade \
		--install \
		--create-namespace \
		--namespace ingress-nginx \
		--wait \
		--timeout $(TIMEOUT) \
		--set controller.allowSnippetAnnotations=true \
		--set controller.service.type=LoadBalancer \
		--set controller.service.nodePorts.http=32080 \
		--set controller.service.nodePorts.https=32443 \
		--set controller.config.proxy-body-size=0 \
		--set controller.config.hsts="false" \
		--set controller.watchIngressWithoutClass=true \
		--set controller.ingressClassResource.default=true \
		--version=$(INGRESS_VERSION) \
		ingress-nginx \
		ingress-nginx/ingress-nginx

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef