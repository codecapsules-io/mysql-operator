#!/usr/bin/env bash
# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Pull an upstream image, retag to a simple Kind-local reference, load into a
# kind cluster, and verify the exact ref is present on the node.
#
# Kind/containerd matches images by exact reference; never load digest-pinned
# refs directly — always retag to kind_ref first.
#
# Usage: hack/kind-load-image.sh CLUSTER_NAME UPSTREAM_REF KIND_REF [PLATFORM]

set -euo pipefail

cluster_name="${1:?cluster name required}"
upstream_ref="${2:?upstream image reference required}"
kind_ref="${3:?kind-local image reference required}"
platform="${4:-linux/amd64}"
node="${cluster_name}-control-plane"

# After docker pull, return the ref docker stores locally (RepoDigest for pins, RepoTag otherwise).
docker_local_ref() {
	local ref="$1"
	local repo_digest repo_tag

	repo_digest="$(docker image inspect "${ref}" --format '{{index .RepoDigests 0}}' 2>/dev/null || true)"
	if [[ -n "${repo_digest}" && "${repo_digest}" != "<no value>" ]]; then
		echo "${repo_digest}"
		return 0
	fi

	repo_tag="$(docker image inspect "${ref}" --format '{{index .RepoTags 0}}' 2>/dev/null || true)"
	if [[ -n "${repo_tag}" && "${repo_tag}" != "<no value>" ]]; then
		echo "${repo_tag}"
		return 0
	fi

	echo "${ref}"
}

load_into_kind() {
	local ref="$1"
	local archive

	echo "loading ${ref} into kind cluster ${cluster_name} via image-archive (${platform})"
	archive="$(mktemp -t kind-image.XXXXXX.tar)"
	docker save --platform "${platform}" -o "${archive}" "${ref}"
	kind load image-archive --name "${cluster_name}" "${archive}"
	rm -f "${archive}"
}

# crictl stores short refs under docker.io/library/ or docker.io/; kubelet resolves the same way.
containerd_image_ref_aliases() {
	local ref="$1"
	local name tag host
	name="${ref%:*}"
	tag="${ref##*:}"
	printf '%s\n' "${ref}"
	if [[ "${name}" != */* ]]; then
		printf 'docker.io/library/%s:%s\n' "${name}" "${tag}"
		return 0
	fi
	host="${name%%/*}"
	case "${host}" in
	*.* | *:* | localhost) return 0 ;;
	esac
	printf 'docker.io/%s:%s\n' "${name}" "${tag}"
}

verify_image_present() {
	local ref="$1"
	local alias json table_lines
	json="$(docker exec "${node}" crictl images -o json 2>/dev/null || true)"
	table_lines="$(docker exec "${node}" crictl images 2>/dev/null | tail -n +2 | awk '{print $1":"$2}' || true)"
	while IFS= read -r alias; do
		[[ -z "${alias}" ]] && continue
		if [[ -n "${json}" ]] && grep -Fq "\"${alias}\"" <<< "${json}"; then
			echo "verified ${ref} on ${node} (as ${alias})"
			return 0
		fi
		if grep -qxF "${alias}" <<< "${table_lines}"; then
			echo "verified ${ref} on ${node} (as ${alias})"
			return 0
		fi
	done < <(containerd_image_ref_aliases "${ref}")
	echo "error: ${ref} not found on kind node ${node} after load" >&2
	docker exec "${node}" crictl images >&2 || true
	return 1
}

echo "pulling ${upstream_ref} (${platform})"
if [[ "${upstream_ref}" == "${kind_ref}" ]]; then
	echo "using pre-built local image ${kind_ref}"
else
	docker pull --platform "${platform}" "${upstream_ref}"
	local_ref="$(docker_local_ref "${upstream_ref}")"
	docker tag "${local_ref}" "${kind_ref}"
fi

load_into_kind "${kind_ref}"
verify_image_present "${kind_ref}"
