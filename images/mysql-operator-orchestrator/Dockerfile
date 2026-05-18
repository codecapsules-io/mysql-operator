# Docker image for orchestrator
#
# Percona/orchestrator fork (pinned ref) includes MySQL 8.4+ replication SQL (SHOW REPLICA STATUS, etc.).
# Upstream openark/orchestrator v3.2.6 issues SHOW SLAVE STATUS / SHOW MASTER STATUS, which MySQL 8.4 rejects (ER_PARSE_ERROR near 'slave status' / 'master status').

# Percona/orchestrator go.mod requires go >= 1.25.x (toolchain); keep in sync with upstream.
FROM golang:1.25-bookworm AS orchestrator-build

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		git gcc libc6-dev rsync perl ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src
# https://github.com/percona/orchestrator — DISTMYSQL-440 / replica terminology for MySQL 8.3+
ARG ORCHESTRATOR_GIT_REF=1982fdc851bb91e66497bcc9b4710a6247ca3ed7
RUN git clone https://github.com/percona/orchestrator.git . \
	&& git checkout "${ORCHESTRATOR_GIT_REF}"

RUN ./script/build

# bookworm-built orchestrator links against glibc >= 2.32; distroless debian11 is too old.
# :debug provides busybox sh + wget for the RUN lines below (same pattern as before).
FROM gcr.io/distroless/base-debian12:debug@sha256:ba851612761364a47a05b365425d7b2eb2c3917f9cb0307d8c962e9790524d39
SHELL ["/busybox/sh", "-c"]

# switch to root for installing software
USER root

RUN set -ex \
	&& mkdir -p /usr/local/bin \
	&& export DOCKERIZE_VERSION=0.6.1 \
	&& wget https://github.com/jwilder/dockerize/releases/download/v${DOCKERIZE_VERSION}/dockerize-linux-amd64-v${DOCKERIZE_VERSION}.tar.gz -O- | \
		tar -C /usr/local/bin -xzv

RUN mkdir -p /usr/local/orchestrator
COPY --from=orchestrator-build /src/bin/orchestrator /usr/local/orchestrator/orchestrator
COPY --from=orchestrator-build /src/bin/resources /usr/local/orchestrator/resources

COPY rootfs/ /
RUN set -ex \
	&& mkdir -p /etc/orchestrator /var/lib/orchestrator \
	&& chown -R 65532:65532 /etc/orchestrator /var/lib/orchestrator /usr/local/orchestrator

# switch back to nonroot for runtime
USER 65532
EXPOSE 3000 10008
VOLUME [ "/var/lib/orchestrator" ]

WORKDIR "/usr/local/orchestrator"

ENTRYPOINT [ "/usr/local/bin/dockerize", \
             "-no-overwrite", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orchestrator.conf.json:/etc/orchestrator/orchestrator.conf.json", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orc-topology.cnf:/etc/orchestrator/orc-topology.cnf", \
             "--" ]
CMD ["/usr/local/orchestrator/orchestrator", "-config", "/etc/orchestrator/orchestrator.conf.json", "http"]
