# Version profiles

Server **images** are resolved by the image resolver (see [MySQL versions & upgrades](mysql-versions-and-upgrades.md)): `spec.image`, CLI overrides, catalog file, then built-in constants.

**Profiles** describe version-line behavior: replication SQL dialect, grant metadata, operator `my.cnf` fragments, sidecar logical key, and validation. Built-in Percona profiles are:

| Profile key | MySQL line |
|-------------|------------|
| `percona-5.7` | 5.7.x |
| `percona-8.0` | 8.0.x through 8.3.x |
| `percona-8.4` | 8.4 LTS and newer |

Unrecognized semvers fall back to `fallback-unknown` with limited guarantees.

## Sidecar mapping

Each profile maps to a dedicated sidecar image configured on the operator:

| Profile | Operator flag |
|---------|---------------|
| `percona-5.7` | `--sidecar-image` |
| `percona-8.0` | `--sidecar-mysql8-image` |
| `percona-8.4` | `--sidecar-mysql84-image` |

There is **no cross-profile fallback**. If you run 8.4 clusters, you must set `--sidecar-mysql84-image` on the operator (included in `deploy/manifests/v0.7.0` defaults).

Override per cluster with `spec.sidecarImage` when needed.

## Orchestrator and mysqld_exporter

Discovery and failover depend on the **Orchestrator** build in this repository's Docker image (`images/mysql-operator-orchestrator/Dockerfile`), built from **[percona/orchestrator](https://github.com/percona/orchestrator)** at a pinned commit so topology discovery uses **MySQL 8.4+ replication SQL** (`SHOW REPLICA STATUS`, etc.). Older openark/orchestrator 3.2.x binaries hit parse errors on 8.4.

Override the orchestrator container image in the operator deployment if you use a different build — see [Install the operator](install-operator.md).

The **mysqld_exporter** image is set with `--metrics-exporter-image` (default `prom/mysqld-exporter:v0.16.0` in versioned manifests). See [Monitoring](monitoring.md).

## Related pages

- [MySQL versions & upgrades](mysql-versions-and-upgrades.md)
- [Operator configuration](operator-configuration.md)
