#!/usr/bin/env bash
# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Pull a container image and load it into a kind cluster under every name variant
# the kubelet may resolve (kind/containerd match by exact image reference).
#
# Usage: hack/kind-load-image.sh CLUSTER_NAME IMAGE [PLATFORM]

set -euo pipefail

cluster_name="${1:?cluster name required}"
image="${2:?image reference required}"
platform="${3:-linux/amd64}"
node="${cluster_name}-control-plane"
ctr_ns="k8s.io"

# Emit deduplicated alias refs for a pulled image.
image_aliases() {
	local ref="$1"
	local -a aliases=("$ref")

	case "${ref}" in
	docker.io/*)
		aliases+=("${ref#docker.io/}")
		;;
	*@sha256:*)
		aliases+=("docker.io/library/${ref}")
		;;
	*/*)
		aliases+=("docker.io/${ref}")
		;;
	*)
		aliases+=("docker.io/library/${ref}")
		;;
	esac

	printf '%s\n' "${aliases[@]}" | awk '!seen[$0]++'
}

is_digest_ref() {
	[[ "$1" == *@sha256:* ]]
}

ctr_images() {
	docker exec "${node}" ctr -n "${ctr_ns}" images ls -q 2>/dev/null || true
}

find_src_ref() {
	local ref needle digest
	for ref in ${image}; do
		if ctr_images | grep -qxF "${ref}"; then
			echo "${ref}"
			return 0
		fi
	done
	while IFS= read -r ref; do
		if ctr_images | grep -qxF "${ref}"; then
			echo "${ref}"
			return 0
		fi
	done < <(image_aliases "${image}")

	if is_digest_ref "${image}"; then
		digest="${image##*@}"
		needle="$(ctr_images | grep -F "${digest}" | head -n 1 || true)"
		if [[ -n "${needle}" ]]; then
			echo "${needle}"
			return 0
		fi
	fi

	return 1
}

load_into_kind() {
	local ref="$1"
	if kind load docker-image --name "${cluster_name}" "${ref}" 2>/dev/null; then
		echo "loaded ${ref} into kind cluster ${cluster_name}"
		return 0
	fi

	echo "kind load docker-image failed for ${ref}; trying image-archive import..." >&2
	local archive
	archive="$(mktemp -t kind-image.XXXXXX.tar)"
	docker save --platform "${platform}" -o "${archive}" "${ref}"
	kind load image-archive --name "${cluster_name}" "${archive}"
	rm -f "${archive}"
	echo "loaded ${ref} via archive into kind cluster ${cluster_name}"
}

tag_in_kind() {
	local src="$1" dst="$2"
	[[ "${src}" == "${dst}" ]] && return 0
	docker exec "${node}" ctr -n "${ctr_ns}" images tag "${src}" "${dst}"
	echo "tagged ${src} -> ${dst} in ${cluster_name}"
}

echo "pulling ${image} (${platform})"
docker pull --platform "${platform}" "${image}"

load_into_kind "${image}"

src_ref="$(find_src_ref)" || {
	echo "error: ${image} not found in kind node ${node} after load" >&2
	exit 1
}

while IFS= read -r alias; do
	[[ "${alias}" == "${src_ref}" ]] && continue
	tag_in_kind "${src_ref}" "${alias}"
done < <(image_aliases "${image}")
