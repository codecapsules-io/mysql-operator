# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
###############################################################################
#  Docker image for sidecar containers
###############################################################################

# Use a supported Debian release (bookworm) for reliable mirrors; override with DEBIAN_RELEASE if needed.
ARG DEBIAN_RELEASE=bookworm
FROM debian:${DEBIAN_RELEASE}-slim AS sidecar

ENV DEBIAN_FRONTEND=noninteractive

RUN printf 'Acquire::Retries "5";\n' >/etc/apt/apt.conf.d/80retry

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    bash ca-certificates curl gnupg lsb-release passwd pigz unzip wget \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 999 mysql
RUN useradd -u 999 -r -g 999 -s /sbin/nologin \
    -c "Default Application User" mysql

COPY rootfs/ /

RUN export RCLONE_VERSION=1.57.0 \
    && gpg --import /etc/gpg-keys/rclone.gpg \
    && wget -nv https://github.com/ncw/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.zip \
    && wget -nv https://github.com/ncw/rclone/releases/download/v${RCLONE_VERSION}/SHA256SUMS \
    && gpg --verify --output=- SHA256SUMS > sums \
    && sha256sum -c --ignore-missing sums \
    && unzip rclone-*-linux-amd64.zip \
    && mv rclone-*-linux-amd64/rclone /usr/local/bin/ \
    && chmod 755 /usr/local/bin/rclone \
    && rm -r rclone-*-linux-amd64 rclone-*-linux-amd64.zip

# Register Percona signing key and apt sources (raw deb line alone is not enough on modern apt).
# Register Percona signing key and apt sources.
# - percona-toolkit lives in the 'pt' repo (the 'tools' repo is deprecated and stuck at 3.6.0)
# - percona-xtrabackup-24 lives in the legacy 'tools' repo
# - percona-xtrabackup-80 lives in the 'pxb-80' repo
# - percona-xtrabackup-84 lives in the 'pxb-84-lts' repo
ARG XTRABACKUP_PKG=percona-xtrabackup-24
RUN wget -nv -O /tmp/percona-release.deb https://repo.percona.com/apt/percona-release_latest.generic_all.deb \
    && dpkg -i /tmp/percona-release.deb \
    && rm -f /tmp/percona-release.deb \
    && percona-release enable pt release \
    && case "${XTRABACKUP_PKG}" in \
    percona-xtrabackup-24) percona-release enable tools release ;; \
    percona-xtrabackup-80) percona-release enable pxb-80 release ;; \
    percona-xtrabackup-84) percona-release enable pxb-84-lts release ;; \
    *) echo "ERROR: Unknown XTRABACKUP_PKG '${XTRABACKUP_PKG}'" >&2; exit 1 ;; \
    esac \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    percona-toolkit ${XTRABACKUP_PKG} unzip default-mysql-client \
    && rm -rf /var/lib/apt/lists/*

USER mysql
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
