# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Project Setup
PROJECT_NAME := mysql-operator
PROJECT_REPO := github.com/codecapsules-io/mysql-operator

PLATFORMS := darwin_amd64 linux_amd64
include build/makelib/common.mk

GO111MODULE=on
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/mysql-operator $(GO_PROJECT)/cmd/mysql-operator-sidecar $(GO_PROJECT)/cmd/orc-helper
# golang.mk accepts any toolchain >= this (go.mod language version may stay older).
GO_MIN_VERSION = 1.17
GOFMT_VERSION = 1.17
GOLANGCI_LINT_VERSION = 2.12.2
GO_LDFLAGS += \
	       -X $(GO_PROJECT)/pkg/version.buildDate=$(BUILD_DATE) \
	       -X $(GO_PROJECT)/pkg/version.gitVersion=$(VERSION) \
	       -X $(GO_PROJECT)/pkg/version.gitCommit=$(GIT_COMMIT) \
	       -X $(GO_PROJECT)/pkg/version.gitTreeState=$(GIT_TREE_STATE)
GO_INTEGRATION_TESTS_SUBDIRS = test/e2e
ifeq ($(CI),true)
E2E_IMAGE_REGISTRY ?= $(DOCKER_REGISTRY)
E2E_IMAGE_TAG ?= $(COMMIT_HASH)
GO_LINT_ARGS += --timeout 15m
else
E2E_IMAGE_REGISTRY ?= docker.io/$(BUILD_REGISTRY)
E2E_IMAGE_TAG ?= latest
E2E_IMAGE_SUFFIX ?= -$(ARCH)
endif
GO_INTEGRATION_TESTS_PARAMS ?= -timeout 50m \
							   -ginkgo.slowSpecThreshold 300 \
							   -- \
							   --pod-wait-timeout 200 \
							   --kubernetes-config $(HOME)/.kube/config \
							   --operator-image $(E2E_IMAGE_REGISTRY)/mysql-operator$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --sidecar-mysql57-image $(E2E_IMAGE_REGISTRY)/mysql-operator-sidecar-5.7$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --sidecar-mysql8-image $(E2E_IMAGE_REGISTRY)/mysql-operator-sidecar-8.0$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --sidecar-mysql84-image $(E2E_IMAGE_REGISTRY)/mysql-operator-sidecar-8.4$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG) \
							   --orchestrator-image $(E2E_IMAGE_REGISTRY)/mysql-operator-orchestrator$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG)
TEST_FILTER_PARAM += $(GO_INTEGRATION_TESTS_PARAMS)
include build/makelib/golang.mk

DOCKER_REGISTRY ?= docker.io/codecapsules-io
IMAGES ?= mysql-operator mysql-operator-orchestrator mysql-operator-sidecar-5.7 mysql-operator-sidecar-8.0 mysql-operator-sidecar-8.4
include build/makelib/image.mk

KUBEBUILDER_ASSETS_VERSION := 1.23.5
# preserveUnknownFields is applied post-generate via .kubebuilder.fix-preserve-unknown-fields (yq).
GEN_CRD_OPTIONS := crd:crdVersions=v1
include build/makelib/kubebuilder-v3.mk

# fix for https://github.com/kubernetes-sigs/controller-tools/issues/476
.PHONY: .kubebuilder.fix-preserve-unknown-fields
.kubebuilder.fix-preserve-unknown-fields: $(YQ)
		@for crd in $(wildcard $(CRD_DIR)/*.yaml) ; do \
			$(YQ) e '.spec.preserveUnknownFields=false' -i "$${crd}" ;\
		done
.PHONY: .kubebuilder.fix-license-headers
.kubebuilder.fix-license-headers:
		@for f in $(wildcard $(CRD_DIR)/*.yaml) config/rbac/role.yaml ; do \
			test -f "$${f}" || continue ; \
			head -n 5 "$${f}" | grep -q 'SPDX-License-Identifier' && continue ; \
			{ echo '# Copyright 2026 Code Capsules' ; \
			  echo '# SPDX-License-Identifier: Apache-2.0' ; \
			  echo '#' ; \
			  cat "$${f}" ; } > "$${f}.tmp" && mv "$${f}.tmp" "$${f}" ; \
		done
.kubebuilder.manifests.done: .kubebuilder.fix-preserve-unknown-fields .kubebuilder.fix-license-headers

DEPLOY_MANIFESTS_DIR ?= deploy/manifests

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: deploy.crds deploy.manifests
deploy.crds: $(YQ) kubebuilder.manifests
	@$(INFO) syncing CRDs into deploy/manifests/$(VERSION)
	@VERSION=$(VERSION) YQ=$(YQ) bash hack/generate-deploy-manifests.sh $(VERSION)
	@$(OK) syncing CRDs into deploy/manifests/$(VERSION)

# Operator manifests are maintained manually per version; this target only syncs CRDs.
deploy.manifests: deploy.crds

.PHONY: validate-domain
validate-domain:
	@$(INFO) validating domain metadata consistency
	@bash hack/validate-domain.sh
	@$(OK) validating domain metadata consistency

.lint.run: validate-domain go.fmt.verify go.lint

CLUSTER_NAME ?= mysql-operator
delete-environment:
	-@kind delete cluster --name $(CLUSTER_NAME)

create-environment: delete-environment
	@kind create cluster --name $(CLUSTER_NAME)
	@$(MAKE) kind-load-images

kind-load-images:
	@set -e; \
		for image in $(IMAGES); do \
		kind load docker-image --name $(CLUSTER_NAME) $(E2E_IMAGE_REGISTRY)/$${image}$(E2E_IMAGE_SUFFIX):$(E2E_IMAGE_TAG); \
	done
