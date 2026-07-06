<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Storage and configuration

This page covers how the operator stores MySQL data and how you can tune server configuration.

## Data volumes (`spec.volumeSpec`)

The operator supports three volume sources for MySQL data. **PersistentVolumeClaim** is the recommended choice for production.

### PersistentVolumeClaim (default)

If you omit `volumeSpec`, the operator creates a PVC of **1Gi** with `ReadWriteOnce` access.

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  secretName: my-secret
  volumeSpec:
    persistentVolumeClaim:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 20Gi
      storageClassName: fast-ssd
```

Your cluster needs a **default StorageClass** or an explicit `storageClassName`. Without one, PVCs stay pending.

### Retain PVCs on cluster delete

Set `keepAfterDelete` when you want data volumes to survive deletion of the `MysqlCluster`:

```yaml
spec:
  volumeSpec:
    keepAfterDelete: true
    persistentVolumeClaim:
      resources:
        requests:
          storage: 20Gi
```

!!! warning
    Retained PVCs are not automatically reattached to a new cluster. You must manage reuse manually.

### HostPath

For development or single-node setups:

```yaml
spec:
  volumeSpec:
    hostPath:
      path: /data/mysql/my-cluster
      type: DirectoryOrCreate
```

You may need an init container to fix permissions — see `podSpec.initContainers` in [`examples/example-cluster.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-cluster.yaml).

### EmptyDir

```yaml
spec:
  volumeSpec:
    emptyDir: {}
```

EmptyDir is **not recommended** for production. Data is lost when pods are rescheduled. Support has been limited since operator v0.3.x.

## MySQL configuration (`spec.mysqlConf`)

The operator generates a `my.cnf` ConfigMap with sensible defaults based on requested pod memory (for example `innodb-buffer-pool-size`). Values in `mysqlConf` **override** those defaults.

```yaml
spec:
  mysqlConf:
    max_allowed_packet: 128M
    innodb-buffer-size: 256M
```

Keys use the same names as MySQL variables (hyphens or underscores as in your `my.cnf`).

During a **version upgrade**, the ConfigMap follows the rollout target version so new pods receive a compatible configuration. See [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Pod specification (`spec.podSpec`)

Common overrides:

| Field | Use |
|-------|-----|
| `resources` | CPU and memory requests/limits for the MySQL container |
| `affinity` / `nodeSelector` / `tolerations` | Scheduling |
| `imagePullSecrets` | Private registry credentials |
| `mysqlLifecycle` | Custom preStop hook (default includes demote-on-shutdown when enabled) |
| `initContainers` | Extra init work (for example volume permissions) |
| `containers` | Additional sidecars |
| `metricsExporterResources` | Resources for mysqld_exporter |
| `mysqlOperatorSidecarResources` | Resources for the operator sidecar |

## Query limits (`spec.queryLimits`)

The operator can run pt-kill to terminate problematic queries:

```yaml
spec:
  queryLimits:
    maxQueryTime: 300
    maxIdleTime: 60
    kill: oldest
    killMode: query
```

See the [pt-kill documentation](https://www.percona.com/doc/percona-toolkit/LATEST/pt-kill.html) for flag semantics.

## Read-only mode (`spec.readOnly`)

When `readOnly: true`, the operator attempts to make the cluster read-only. During failover there can be a short window where writes are still accepted.

## Related pages

- [MysqlCluster](mysql-cluster.md) — full API reference
- [MySQL version profiles](mysql-version-profiles.md) — version-line defaults in `my.cnf`
