# Code Capsules MySQL Operator

The **Code Capsules MySQL Operator** is a Kubernetes controller for running highly available MySQL clusters. You can declare clusters, users, and databases as custom resources; the operator handles replication, failover, and day-to-day operations.

This project is an independently maintained fork of the open-source [bitpoke/mysql-operator](https://github.com/bitpoke/mysql-operator). It does **not** track upstream Bitpoke releases, Helm charts, or documentation.

## What you can do with this operator

- Deploy **Percona Server for MySQL** clusters (5.7, 8.0, and 8.4 LTS lines).
- Run **multi-replica HA** with automatic failover via [Orchestrator](orchestrator.md).
- Manage **users and databases** declaratively with `MysqlUser` and `MysqlDatabase` resources.
- Upgrade MySQL versions in a controlled way with operator orchestration (see [MySQL versions & upgrades](mysql-versions-and-upgrades.md)).

Built-in backup and restore features exist in the codebase but are **not actively maintained** for new deployments. See [Legacy backups](legacy-backups.md) and the [maintenance scope](https://github.com/codecapsules-io/mysql-operator/blob/master/MAINTENANCE.md).

## Quick start

Install the operator from versioned manifests:

```shell
export OPERATOR_VERSION=v0.7.0
kubectl create namespace mysql-operator --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

Deploy a sample cluster:

```shell
kubectl apply -f examples/example-cluster-secret.yaml
kubectl apply -f examples/example-cluster.yaml
```

For a full walkthrough, see [Getting started](getting-started.md).

## Documentation map

| Topic | Page |
|-------|------|
| First-time install and first cluster | [Getting started](getting-started.md) |
| Operator install, customize, upgrade | [Install the operator](install-operator.md) |
| Move off upstream Helm | [Migrating from Helm](migrate-from-helm.md) |
| `MysqlCluster` API reference | [MysqlCluster](mysql-cluster.md) |
| PVCs, `mysqlConf`, tuning | [Storage & configuration](storage-and-configuration.md) |
| Services and app connection strings | [Connecting applications](connecting-applications.md) |
| MySQL 8.4 and version upgrades | [MySQL versions & upgrades](mysql-versions-and-upgrades.md) |
| Prometheus metrics | [Monitoring](monitoring.md) |

## Technology

Clusters use **Percona Server for MySQL** for operational features (XtraBackup tooling in sidecars, monitoring integration). Failover topology is managed by **Percona Orchestrator**, bundled in the operator StatefulSet.

## License

Apache License 2.0. See [LICENSE](https://github.com/codecapsules-io/mysql-operator/blob/master/LICENSE) and [NOTICE](https://github.com/codecapsules-io/mysql-operator/blob/master/NOTICE) in the repository.
