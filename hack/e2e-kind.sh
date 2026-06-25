#!/usr/bin/env bash
# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Run the Kind-based e2e flow (same contract as .github/workflows/e2e.yml).
#
# Image contract (kind load + IfNotPresent — see kind.sigs.k8s.io docs):
#   - Locally built operator/sidecar/orchestrator images: tag :local (never :latest),
#     kind load docker-image, referenced with imagePullPolicy IfNotPresent in e2e patches.
#   - Public images (Percona server, mysqld-exporter): NOT preloaded; kind nodes pull
#     from Docker Hub when cluster pods are created (GHA runners have network).
#
# Usage:
#   hack/e2e-kind.sh                 # create cluster, build, load images, run all cluster e2e tests
#   hack/e2e-kind.sh --focus 'scales up'   # run a single spec (faster smoke test)
#   hack/e2e-kind.sh --build-only       # build images only (CI: before kind create)
#   hack/e2e-kind.sh --skip-build    # reuse existing images
#   hack/e2e-kind.sh --skip-cluster  # reuse existing kind cluster (CI: cluster from helm/kind-action)
#   hack/e2e-kind.sh --down          # delete the kind cluster and exit
#
# Requires: docker (running), kind, kubectl, go, make

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

export PATH="${PATH}:${HOME}/go/bin:/usr/local/go/bin:/opt/homebrew/bin"

cpu_count() {
	getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4
}

CLUSTER_NAME="${CLUSTER_NAME:-mysql-operator}"
E2E_IMAGE_REGISTRY="${E2E_IMAGE_REGISTRY:-mysql-operator-ci}"
E2E_IMAGE_TAG="${E2E_IMAGE_TAG:-local}"
CI_BUILD_NUMBER="${CI_BUILD_NUMBER:-local}"
POD_WAIT_TIMEOUT="${POD_WAIT_TIMEOUT:-600}"
EXPORTER_IMAGE="${EXPORTER_IMAGE:-prom/mysqld-exporter:v0.16.0}"

# Built locally by make — the only images we kind load.
LOCAL_IMAGES=(
	mysql-operator
	mysql-operator-orchestrator
	mysql-operator-sidecar-5.7
	mysql-operator-sidecar-8.0
	mysql-operator-sidecar-8.4
)

SKIP_BUILD=0
SKIP_CLUSTER=0
BUILD_ONLY=0
DOWN_ONLY=0
GINKGO_FOCUS=""
GINKGO_SKIP="Mysql backups"

usage() {
	sed -n '2,18p' "$0" | tr -d '#'
}

require_cmd() {
	for cmd in "$@"; do
		if ! command -v "${cmd}" >/dev/null 2>&1; then
			echo "error: '${cmd}' not found in PATH" >&2
			case "${cmd}" in
			kind) echo "  install: brew install kind   (macOS) or go install sigs.k8s.io/kind@v0.24.0" >&2 ;;
			esac
			exit 1
		fi
	done
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	--skip-build)
		SKIP_BUILD=1
		shift
		;;
	--build-only)
		BUILD_ONLY=1
		SKIP_CLUSTER=1
		shift
		;;
	--skip-cluster)
		SKIP_CLUSTER=1
		shift
		;;
	--down)
		DOWN_ONLY=1
		shift
		;;
	--focus)
		GINKGO_FOCUS="$2"
		shift 2
		;;
	--skip-backups)
		GINKGO_SKIP="Mysql backups"
		shift
		;;
	*)
		echo "unknown argument: $1" >&2
		usage
		exit 1
		;;
	esac
done

require_cmd docker kind kubectl go make

if ! docker info >/dev/null 2>&1; then
	echo "error: docker is not running" >&2
	exit 1
fi

if [[ "${DOWN_ONLY}" -eq 1 ]]; then
	kind delete cluster --name "${CLUSTER_NAME}" || true
	exit 0
fi

create_cluster() {
	if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
		echo "kind cluster '${CLUSTER_NAME}' already exists"
		return
	fi
	kind create cluster --name "${CLUSTER_NAME}"
}

kind_node_platform() {
	if [[ -n "${KIND_NODE_PLATFORM:-}" ]]; then
		echo "${KIND_NODE_PLATFORM}"
		return 0
	fi
	local arch
	arch="$(docker exec "${CLUSTER_NAME}-control-plane" uname -m)"
	case "${arch}" in
	x86_64) echo "linux/amd64" ;;
	aarch64 | arm64) echo "linux/arm64" ;;
	*) echo "linux/amd64" ;;
	esac
}

build_images() {
	local platform make_platform arch_suffix
	platform="$(kind_node_platform)"
	make_platform="${platform//\//_}"
	arch_suffix="${make_platform#linux_}"

	export BUILD_ARGS="${BUILD_ARGS:---load}"
	export CI=true
	export CI_BUILD_NUMBER="${CI_BUILD_NUMBER}"
	export IMAGE_TAG="${E2E_IMAGE_TAG}"
	export DOCKER_REGISTRY="${E2E_IMAGE_REGISTRY}"

	echo "building operator images for ${platform}"
	local jobs
	jobs="$(cpu_count)"
	if [[ "${CI:-}" == "true" ]]; then
		# Limit parallel compile/link on GHA to leave RAM for kind + MySQL pods.
		jobs=2
	fi
	make -j"${jobs}" build \
		PLATFORMS="${make_platform}" BUILD_PLATFORMS="${make_platform}"
	make .img.release.build

	for img in "${LOCAL_IMAGES[@]}"; do
		built="${E2E_IMAGE_REGISTRY}/${img}:${E2E_IMAGE_TAG}-${arch_suffix}"
		ref="${E2E_IMAGE_REGISTRY}/${img}:${E2E_IMAGE_TAG}"
		docker tag "${built}" "${ref}"
		docker tag "${ref}" "${img}:${E2E_IMAGE_TAG}"
	done
}

load_local_images() {
	local img kind_ref node="${CLUSTER_NAME}-control-plane" json
	for img in "${LOCAL_IMAGES[@]}"; do
		kind_ref="${img}:${E2E_IMAGE_TAG}"
		echo "loading ${kind_ref} into kind cluster ${CLUSTER_NAME}"
		kind load docker-image --name "${CLUSTER_NAME}" "${kind_ref}"
		json="$(docker exec "${node}" crictl images -o json 2>/dev/null || true)"
		if ! grep -Fq "${kind_ref}" <<< "${json}" \
			&& ! grep -Fq "library/${kind_ref}" <<< "${json}"; then
			echo "error: ${kind_ref} not visible on ${node} after kind load" >&2
			docker exec "${node}" crictl images >&2 || true
			exit 1
		fi
		echo "verified ${kind_ref} on ${node}"
	done
	echo "locally built images on ${CLUSTER_NAME}-control-plane:"
	docker exec "${CLUSTER_NAME}-control-plane" crictl images \
		| grep -E 'mysql-operator|REPOSITORY' || true
}

run_e2e() {
	local ginkgo_args=(-timeout 75m -ginkgo.slowSpecThreshold 600)
	if [[ -n "${GINKGO_SKIP}" ]]; then
		ginkgo_args+=(-ginkgo.skip="${GINKGO_SKIP}")
	fi
	if [[ -n "${GINKGO_FOCUS}" ]]; then
		ginkgo_args+=(-ginkgo.focus="${GINKGO_FOCUS}")
	fi

	local test_args=(
		--pod-wait-timeout "${POD_WAIT_TIMEOUT}"
		--kubernetes-config "${HOME}/.kube/config"
		--operator-image "mysql-operator:${E2E_IMAGE_TAG}"
		--sidecar-mysql57-image "mysql-operator-sidecar-5.7:${E2E_IMAGE_TAG}"
		--sidecar-mysql8-image "mysql-operator-sidecar-8.0:${E2E_IMAGE_TAG}"
		--sidecar-mysql84-image "mysql-operator-sidecar-8.4:${E2E_IMAGE_TAG}"
		--orchestrator-image "mysql-operator-orchestrator:${E2E_IMAGE_TAG}"
		--metrics-exporter-image "${EXPORTER_IMAGE}"
		--verify-kind-local-images
	)

	local params="${ginkgo_args[*]} -- ${test_args[*]}"
	echo "Running: make e2e GO_INTEGRATION_TESTS_PARAMS=\"${params}\""

	ACK_GINKGO_DEPRECATIONS=1.16.4 \
		make e2e GO_INTEGRATION_TESTS_PARAMS="${params}"
}

if [[ "${DOWN_ONLY}" -eq 1 ]]; then
	kind delete cluster --name "${CLUSTER_NAME}" || true
	exit 0
fi

if [[ "${BUILD_ONLY}" -eq 1 ]]; then
	build_images
	exit 0
fi

if [[ "${SKIP_CLUSTER}" -eq 0 ]]; then
	kind delete cluster --name "${CLUSTER_NAME}" 2>/dev/null || true
	create_cluster
fi

if [[ "${SKIP_BUILD}" -eq 0 ]]; then
	build_images
fi

load_local_images
run_e2e
