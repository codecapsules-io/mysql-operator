<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Operator configuration

The operator process accepts command-line flags (and some environment variables) that control images, Orchestrator connectivity, watch scope, and security behavior. Defaults for a release are set in `deploy/manifests/<version>/operator/statefulset.yaml`.

## Image flags

| Flag | Description | Default (v0.7.0 manifests) |
|------|-------------|------------------------------|
| `--sidecar-image` | Sidecar for MySQL 5.7 | `codecapsules-io/mysql-operator-sidecar-5.7:…` |
| `--sidecar-mysql8-image` | Sidecar for MySQL 8.0–8.3 | `codecapsules-io/mysql-operator-sidecar-8.0:…` |
| `--sidecar-mysql84-image` | Sidecar for Percona 8.4 LTS | `codecapsules-io/mysql-operator-sidecar-8.4:…` |
| `--metrics-exporter-image` | mysqld_exporter image | `prom/mysqld-exporter:v0.16.0` |
| `--image-pull-policy` | Default pull policy for cluster pods | `IfNotPresent` |
| `--image-pull-secret` | Default pull secret name | (empty) |

Environment variable overrides exist for sidecar defaults: `MYSQL_OPERATOR_SIDECAR_MYSQL57_IMAGE`, `MYSQL_OPERATOR_SIDECAR_MYSQL8_IMAGE`, `MYSQL_OPERATOR_SIDECAR_MYSQL84_IMAGE`.

## MySQL version catalog

| Flag | Description |
|------|-------------|
| `--mysql-versions-to-image` | Map of `semver=image` overrides (for example `8.4.8=percona:8.4.8`) |
| `--mysql-version-catalog-file` | Path to a file of `semver=image` lines (merged after built-in defaults) |

See [MySQL versions & upgrades](mysql-versions-and-upgrades.md) for catalog file format and mounting via ConfigMap.

## Orchestrator

| Flag / env | Description |
|------------|-------------|
| `--orchestrator-uri` | HTTP API base URL (default in manifests: `http://mysql-operator.mysql-operator/api`) |
| `--orchestrator-topology-user` / `ORC_TOPOLOGY_USER` | User Orchestrator uses to connect to MySQL pods |
| `--orchestrator-topology-password` / `ORC_TOPOLOGY_PASSWORD` | Password for topology user |
| `--orchestrator-concurrent-reconciles` | Orchestrator controller workers (default `10`) |

Topology credentials are also stored in the `mysql-operator-orc` Secret. Keep them stable across upgrades — see [Install the operator](install-operator.md).

## Scope and election

| Flag | Description |
|------|-------------|
| `--namespace` | Limit the operator to a single namespace (empty = all namespaces) |
| `--leader-election-namespace` | Namespace for leader election lease |
| `--leader-election-id` | Lease resource name (default `mysql-operator-leader-election`) |

## Failover and security

| Flag | Default | Description |
|------|---------|-------------|
| `--failover-before-shutdown` | `true` | Run Orchestrator failover in MySQL preStop hook |
| `--cluster-restrict-privilege-escalation` | `false` | Set `allowPrivilegeEscalation: false` on all cluster pod containers |
| `--allow-cross-namespace-user` | `false` | Allow `MysqlUser` to reference clusters in other namespaces |
| `--allow-cross-namespace-database` | `false` | Allow `MysqlDatabase` to reference clusters in other namespaces |

The privilege-escalation flag is commented out in default v0.7.0 manifests. Enable it when your policy requires stricter pod security contexts.

## Metrics and health

| Flag | Description |
|------|-------------|
| `--metrics-addr` | Operator Prometheus metrics listen address (default `:8080`) |
| `--healthz-addr` | Health probe listen address (default `:8081`) |

Set `--metrics-addr=0` to disable operator metrics.

## Editing flags

1. Check out or fork the repository.
2. Edit `deploy/manifests/<version>/operator/statefulset.yaml` under the `operator` container `args` list.
3. Apply: `kubectl apply -k deploy/manifests/<version>/operator`
4. Wait for rollout: `kubectl rollout status statefulset/mysql-operator -n mysql-operator`

## Related pages

- [Install the operator](install-operator.md)
- [Version profiles](mysql-version-profiles.md)
