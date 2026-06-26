# Legacy backups

The operator includes a built-in backup subsystem: on-demand `MysqlBackup` resources, scheduled backups via `spec.backupSchedule`, and cluster initialization from `spec.initBucketURL`. This functionality is **not actively maintained** in the Code Capsules fork.

!!! warning "Maintenance scope"
    For new deployments, use **external backup solutions** or platform-level backup. See the [maintenance scope](https://github.com/codecapsules-io/mysql-operator/blob/master/MAINTENANCE.md).

This page exists so operators running legacy configurations can understand what the CRDs do and where to find examples.

## What still exists in the codebase

| Resource / field | Purpose |
|------------------|---------|
| `MysqlBackup` CR | Request a one-off backup job |
| `spec.backupSchedule` | Cron-based scheduled backups (6-field cron with seconds) |
| `spec.backupURL` | Remote storage destination (for example `s3://bucket/path/`) |
| `spec.backupSecretName` | Credentials for object storage (Rclone backends) |
| `spec.initBucketURL` | Restore / clone from an existing backup at cluster creation |

Controllers and sidecar tooling for XtraBackup and Rclone remain in the repository but do not receive active feature work.

## Examples

Reference manifests (use at your own risk):

- [`examples/example-backup.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-backup.yaml)
- [`examples/example-backup-secret.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-backup-secret.yaml)
- [`examples/example-cluster-init.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-cluster-init.yaml)

## On-demand backup (legacy)

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlBackup
metadata:
  name: my-cluster-backup
spec:
  clusterName: my-cluster
```

The cluster must have `backupURL` and `backupSecretName` configured.

```shell
kubectl get mysqlbackup
kubectl describe mysqlbackup my-cluster-backup
```

## Scheduled backups (legacy)

```yaml
spec:
  backupSchedule: "0 0 2 * * *"   # daily at 02:00:00
  backupURL: s3://my-bucket/my-cluster/
  backupSecretName: my-cluster-backup-secret
  backupRemoteDeletePolicy: retain
```

Disable scheduled backups by setting `backupSchedule: ""`.

## Restore via new cluster (legacy)

Create a new `MysqlCluster` with:

```yaml
spec:
  initBucketURL: s3://my-bucket/my-cluster/backup.xtrabackup.gz
  initBucketSecretName: my-cluster-backup-secret
```

## Recommendations

- **New clusters:** plan backups outside this operator (cloud provider snapshots, Percona XtraBackup in CronJobs, logical dumps, and similar).
- **Existing clusters:** test restore procedures regularly; do not assume backward compatibility across operator upgrades.
- **MySQL upgrades:** always take a backup before changing `spec.mysqlVersion` — see [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Related pages

- [MysqlCluster](mysql-cluster.md) — backup-related spec fields
- [Maintenance scope](https://github.com/codecapsules-io/mysql-operator/blob/master/MAINTENANCE.md)
