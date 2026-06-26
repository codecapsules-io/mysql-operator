# MysqlCluster

The `MysqlCluster` custom resource (`mysql.presslabs.org/v1alpha1`) describes a highly available MySQL cluster. The operator creates a StatefulSet, Services, secrets, config maps, and related objects from your spec.

## Minimal example

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
  mysqlVersion: "8.0"
```

Apply a bootstrap secret first — see [Getting started](getting-started.md).

!!! tip "Cluster name length"
    Keep the cluster name reasonably short. Very long names can fail to register with Orchestrator.

## Credentials secret (`spec.secretName`)

The secret must exist in the same namespace as the cluster and include at least:

| Key | Required | Purpose |
|-----|----------|---------|
| `ROOT_PASSWORD` | Yes | MySQL root password (base64-encoded) |
| `USER` | No | Application user created at bootstrap |
| `PASSWORD` | No | Password for `USER` |
| `DATABASE` | No | Database created at bootstrap |

All values must be base64-encoded in the Secret `data` field.

The operator also creates a separate **operated secret** (`{cluster}-mysql-operated`) with internal credentials for replication, metrics, Orchestrator, and backups. See [Connecting applications](connecting-applications.md).

## Spec reference

### Core fields

| Field | Description | Default |
|-------|-------------|---------|
| `replicas` | Number of MySQL pods | `0` if unset (set explicitly in production) |
| `secretName` | Bootstrap credentials secret | **Required** |
| `mysqlVersion` | Server version line (`"5.7"`, `"8.0"`, `"8.4"`, or exact semver) | `5.7` |
| `image` | Custom server image (overrides catalog; `mysqlVersion` still required) | From catalog |
| `sidecarImage` | Per-cluster sidecar override | Operator flags |
| `minAvailable` | PodDisruptionBudget minimum available | `50%` |
| `readOnly` | Put cluster in read-only mode (best-effort) | `false` |
| `serverIDOffset` | Offset added to StatefulSet ordinal for `server_id` | `0` |

### Initialization and cloning

| Field | Description |
|-------|-------------|
| `initBucketURL` | Object storage URL with an XtraBackup to seed data (for example `s3://bucket/backup.xtrabackup.gz`) |
| `initBucketSecretName` | Secret with storage credentials for `initBucketURL` |
| `initBucketURI` | **Deprecated** — use `initBucketURL` |

### Storage

| Field | Description |
|-------|-------------|
| `volumeSpec` | PVC, HostPath, or EmptyDir for data; see [Storage & configuration](storage-and-configuration.md) |
| `volumeSpec.keepAfterDelete` | When `true`, retain PVCs after the `MysqlCluster` is deleted |

### MySQL tuning

| Field | Description |
|-------|-------------|
| `mysqlConf` | Key-value map merged into `my.cnf` under `[mysqld]` |
| `podSpec` | Pod template overrides (resources, affinity, extra containers, and more) |
| `maxSlaveLatency` | Remove replicas from read service when lag exceeds this many seconds |
| `queryLimits` | pt-kill parameters for long-running or idle queries |

### Legacy backup fields

These fields drive the built-in backup subsystem, which is **not actively maintained**. See [Legacy backups](legacy-backups.md).

| Field | Description |
|-------|-------------|
| `backupSchedule` | Cron expression with seconds for scheduled backups |
| `backupURL` | Remote storage URL for backups |
| `backupSecretName` | Credentials for backup storage |
| `backupRemoteDeletePolicy` | `retain` or `delete` remote backup data |
| `backupScheduleJobsHistoryLimit` | Number of scheduled backup jobs to keep |
| `backupCompressCommand` / `backupDecompressCommand` | Custom compress/decompress commands |
| `rcloneExtraArgs`, `xbstreamExtraArgs`, `xtrabackupExtraArgs`, etc. | Extra arguments for backup tooling |

## Status reference

| Field | Description |
|-------|-------------|
| `readyNodes` | Count of ready replicas |
| `appliedMysqlVersion` | MySQL version confirmed on all pods via `SELECT VERSION()` |
| `conditions` | Cluster health and operational state |
| `nodes` | Per-node status from Orchestrator |

### Conditions

| Type | Meaning |
|------|---------|
| `Ready` | StatefulSet is ready and cluster is operational |
| `PendingFailoverAck` | Orchestrator is waiting for failover acknowledgment |
| `ReadOnly` | Cluster is in read-only mode |
| `FailoverInProgress` | Orchestrator is performing a failover |
| `UpgradeBlocked` | Requested `mysqlVersion` change is invalid (downgrade or skipped LTS line) |

When `UpgradeBlocked` is `True`, correct `spec.mysqlVersion` to a valid path (for example 8.0 before 8.4). See [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Short names and kubectl

```shell
kubectl get mysql
kubectl describe mysql my-cluster
kubectl get mysqlcluster my-cluster -o yaml
```

## Examples

Commented samples live in the repository:

- [`examples/example-cluster.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-cluster.yaml)
- [`examples/example-cluster-84.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-cluster-84.yaml) — Percona 8.4

## Related pages

- [Storage & configuration](storage-and-configuration.md)
- [Connecting applications](connecting-applications.md)
- [MySQL versions & upgrades](mysql-versions-and-upgrades.md)
