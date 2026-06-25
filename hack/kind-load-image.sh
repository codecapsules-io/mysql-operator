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

load_one() {
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

echo "pulling ${image} (${platform})"
docker pull --platform "${platform}" "${image}"

while IFS= read -r alias; do
	if [[ "${alias}" != "${image}" ]]; then
		docker tag "${image}" "${alias}"
	fi
	load_one "${alias}"
done < <(image_aliases "${image}")
