#!/usr/bin/env bash
# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Run the same Kind-based e2e flow as .github/workflows/e2e.yml locally.
#
# Usage:
#   hack/e2e-kind.sh                 # create cluster, build, load images, run all cluster e2e tests
#   hack/e2e-kind.sh --focus 'scales up'   # run a single spec (faster smoke test)
#   hack/e2e-kind.sh --skip-build    # reuse existing images
#   hack/e2e-kind.sh --skip-cluster  # reuse existing kind cluster
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
# MySQL server images from pkg/util/constants/constants.go (MysqlImageVersions).
# E2e clusters default to 5.7.35 (MySQLDefaultVersion); only that image is required today.
# Sidecars 5.7/8.0/8.4 are built locally; server images are pulled from Docker Hub.
SERVER_IMAGES=(
	"percona@sha256:caab4e854bd75040d07802bf1862bfef1d2b4db0acbc9c4aaf5c21c698fdd393"              # 5.7.35 (default)
	"percona@sha256:6d4524eccd26af7bd7fb623c567159dfbd7f3d9a0e2f7bebd54af1e9ca9903dc"              # 8.0.20
	"docker.io/percona/percona-server@sha256:eaa4cf955f8a01a43faa6ef656bf8fb69a17c17c278a3b0514212291ca0448b1" # 8.4.8
)

IMAGES=(
	mysql-operator
	mysql-operator-orchestrator
	mysql-operator-sidecar-5.7
	mysql-operator-sidecar-8.0
	mysql-operator-sidecar-8.4
)

SKIP_BUILD=0
SKIP_CLUSTER=0
DOWN_ONLY=0
GINKGO_FOCUS=""
GINKGO_SKIP="Mysql backups"

usage() {
	sed -n '2,14p' "$0" | tr -d '#'
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

# Kind nodes are linux/$arch; pull and load a single platform to avoid manifest-list import bugs.
kind_node_platform() {
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

	# Docker images must match the kind node architecture.
	export BUILD_ARGS="${BUILD_ARGS:---load}"
	export CI=true
	export CI_BUILD_NUMBER="${CI_BUILD_NUMBER}"
	export IMAGE_TAG="${E2E_IMAGE_TAG}"
	export DOCKER_REGISTRY="${E2E_IMAGE_REGISTRY}"

	echo "building operator images for ${platform}"
	make -j"$(cpu_count)" build \
		PLATFORMS="${make_platform}" BUILD_PLATFORMS="${make_platform}"
	make .img.release.build

	for img in "${IMAGES[@]}"; do
		built="${E2E_IMAGE_REGISTRY}/${img}:${E2E_IMAGE_TAG}-${arch_suffix}"
		ref="${E2E_IMAGE_REGISTRY}/${img}:${E2E_IMAGE_TAG}"
		docker tag "${built}" "${ref}"
		docker tag "${ref}" "${img}:${E2E_IMAGE_TAG}"
	done
}

load_operator_images() {
	for img in "${IMAGES[@]}"; do
		kind load docker-image --name "${CLUSTER_NAME}" "${img}:${E2E_IMAGE_TAG}"
	done
}

load_external_image() {
	local image="$1"
	local platform
	platform="$(kind_node_platform)"

	echo "loading external image ${image} (${platform})"
	docker pull --platform "${platform}" "${image}"

	if kind load docker-image --name "${CLUSTER_NAME}" "${image}"; then
		return 0
	fi

	echo "kind load docker-image failed for ${image}; trying image-archive import..." >&2
	local archive
	archive="$(mktemp -t kind-image.XXXXXX.tar)"
	if docker save --platform "${platform}" -o "${archive}" "${image}" \
		&& kind load image-archive --name "${CLUSTER_NAME}" "${archive}"; then
		rm -f "${archive}"
		return 0
	fi
	rm -f "${archive}"

	echo "warning: could not preload ${image}; cluster will pull it at runtime (needs network)" >&2
}

load_images() {
	load_operator_images
	for image in "${SERVER_IMAGES[@]}"; do
		load_external_image "${image}"
	done
	load_external_image "${EXPORTER_IMAGE}"
	docker exec "${CLUSTER_NAME}-control-plane" crictl images
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
	)

	local params="${ginkgo_args[*]} -- ${test_args[*]}"
	echo "Running: make e2e GO_INTEGRATION_TESTS_PARAMS=\"${params}\""

	ACK_GINKGO_DEPRECATIONS=1.16.4 \
		make e2e GO_INTEGRATION_TESTS_PARAMS="${params}"
}

if [[ "${SKIP_CLUSTER}" -eq 0 ]]; then
	kind delete cluster --name "${CLUSTER_NAME}" 2>/dev/null || true
	create_cluster
fi

if [[ "${SKIP_BUILD}" -eq 0 ]]; then
	build_images
fi

load_images
run_e2e
