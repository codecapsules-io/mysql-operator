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
	if docker exec "${node}" ctr -n "${ctr_ns}" images tag "${src}" "${dst}" 2>/dev/null; then
		echo "tagged ${src} -> ${dst} in ${cluster_name}"
		return 0
	fi
	return 1
}

echo "pulling ${image} (${platform})"
docker pull --platform "${platform}" "${image}"

# Load using docker's canonical local ref (e.g. docker.io/library/percona@sha256:...).
# kind/containerd stores that name; short refs like percona@sha256:... are added via ctr tag.
load_ref="$(docker_local_ref "${image}")"
load_into_kind "${load_ref}"

src_ref="${load_ref}"
while IFS= read -r alias; do
	[[ "${alias}" == "${src_ref}" ]] && continue
	if tag_in_kind "${src_ref}" "${alias}"; then
		continue
	fi
	# kind may have imported under the requested ref instead of the canonical digest name.
	if tag_in_kind "${image}" "${alias}"; then
		continue
	fi
	echo "warning: could not tag ${alias} in ${cluster_name}; kubelet may pull at runtime" >&2
done < <(image_aliases "${image}")
