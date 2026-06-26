# MySQL versions and upgrades

This operator resolves the Percona (or other) **server image** for a `MysqlCluster` in this order:

1. `spec.image` on the cluster (highest priority).
2. `--mysql-versions-to-image` CLI entries for the exact semver string (for example `8.4.0=...`).
3. Lines from `--mysql-version-catalog-file` (for example a ConfigMap mounted as a file), same `semver=image` format.
4. Built-in defaults in the operator source (`pkg/util/constants/constants.go`).

**Sidecar images** are chosen by resolving the server semver to a **profile** (`percona-5.7`, `percona-8.0`, `percona-8.4`) and mapping that to operator flags (`--sidecar-image`, `--sidecar-mysql8-image`, `--sidecar-mysql84-image`). Each profile uses only its configured image — there is no fallback to another profile's sidecar (for example 8.4 does not use the 8.0 sidecar when `--sidecar-mysql84-image` is empty). You can override with `spec.sidecarImage` on a cluster. Cluster validation rejects a missing sidecar image for the requested version.

Version-specific SQL and `my.cnf` behavior is defined in built-in profiles; see [Version profiles](mysql-version-profiles.md).

## Built-in version catalog

Short tags in `spec.mysqlVersion` map to pinned patch releases:

| Short tag | Resolves to |
|-----------|-------------|
| `"5.7"` | `5.7.35` |
| `"8.0"` | `8.0.20` |
| `"8.4"` | `8.4.8` |

You can also set an exact semver (for example `8.0.34`) when using a custom `spec.image`.

### Short tags vs exact versions

Built-in catalog images accept short tags such as `"8.0"` or `"8.4"`. For **custom `spec.image`**, set `spec.mysqlVersion` to the **exact** server version that image runs (for example `8.0.34`, not `"8.0"`). Init containers, sidecar profile selection, and `MY_MYSQL_VERSION` on pods follow that declaration before mysqld is up. Digest-only images require an explicit `mysqlVersion`. Once pods are running, `status.appliedMysqlVersion` is confirmed by SQL.

## Updating images without rebuilding the operator

1. Mount a ConfigMap (or Secret) into the operator pod at a path such as `/etc/mysql-operator/catalog/versions.properties`.
2. Pass `--mysql-version-catalog-file=/etc/mysql-operator/catalog/versions.properties`.
3. Restart the operator pod so it re-reads the catalog file.

The catalog file is a list of lines `8.4.2=percona@sha256:...` (comments with `#` and blank lines are allowed).

## Operator deployment

Pass `--sidecar-mysql84-image`, `--mysql-version-catalog-file`, and related flags on the operator StatefulSet. Versioned examples are in [`deploy/manifests/`](https://github.com/codecapsules-io/mysql-operator/tree/master/deploy/manifests) (see operator container `args` in `deploy/manifests/v0.7.0/operator/statefulset.yaml`).

## MySQL server major upgrades

Changing `mysqlVersion` / `image` on an existing cluster is a **data plane** operation: follow Percona's upgrade documentation, take backups, and roll instances in a safe order. The operator's `mysql.presslabs.org/version` annotation refers to **operator** schema upgrades, not the MySQL server version.

### Upgrade orchestration

When `spec.mysqlVersion` changes on a cluster that already has data on PVCs, the cluster controller:

1. Compares the desired version to `status.appliedMysqlVersion` (the version **fully running** on the data plane, confirmed by SQL).
2. **Legacy backfill:** clusters with data but empty `status.appliedMysqlVersion` are held until the operator runs unanimous `SELECT VERSION()` on every ready mysqld pod and writes the result to status.
3. Validates the upgrade path (no downgrades; one LTS line at a time, for example 8.0.x before 8.4.x). Invalid paths set the `UpgradeBlocked` condition and keep the cluster on its current version until `spec.mysqlVersion` is corrected.
4. Rolls out the new pod template for valid paths (including any required init containers, for example `mysql-datadir-chown` for Percona 8.0→8.4).
5. Sets `status.appliedMysqlVersion` only after every replica is ready, all init containers succeeded, and unanimous `SELECT VERSION()` reports the desired version.

Patch-level bumps within the same profile line (for example `8.0.20` → `8.0.34`) follow the same rollout gates without extra steps.

#### Cluster `my.cnf` during rollout

The cluster ConfigMap follows the **rollout target version**, not `status.appliedMysqlVersion`. During a rolling upgrade, replicas still on the old mysqld version may briefly see an updated `my.cnf` until they are replaced.

The operator does **not** run pre-upgrade compatibility Jobs (such as `mysqlsh util.checkForServerUpgrade`). Run Percona's upgrade checks and take backups yourself before changing `spec.mysqlVersion`.

For a clean upgrade test: deploy on the source version (for example `8.0`), wait until `status.appliedMysqlVersion` matches and the cluster is Ready, then change `mysqlVersion` to the target (for example `8.4`).

### Auth plugin migration (8.0 → 8.4+, manual prerequisite)

The operator **does not** migrate authentication plugins. Percona/MySQL 8.4+ no longer loads `mysql_native_password`; persistent accounts still using that plugin cannot authenticate after the upgrade.

**Complete this step on the writable primary before changing `spec.mysqlVersion` to 8.4.x.**

Operator utility users (`sys_operator`, `sys_replication`, `sys_exporter`, `sys_heartbeat`, orchestrator topology user) are recreated on every mysqld start via `init-file`. On 8.4 the server default plugin is `caching_sha2_password`, so those accounts do **not** need manual migration.

#### Runbook

1. Take a backup (required for any major upgrade).
2. On the **writable primary**, list persistent accounts still on `mysql_native_password`:

   ```sql
   SELECT user, host, plugin
   FROM mysql.user
   WHERE plugin = 'mysql_native_password'
     AND user NOT IN ('mysql.infoschema', 'mysql.session', 'mysql.sys');
   ```

3. For each row returned, migrate on the primary (replication applies the change to secondaries):

   ```sql
   -- When you know the password:
   ALTER USER 'user'@'host' IDENTIFIED WITH caching_sha2_password BY 'password';

   -- When the password is unknown (for example MysqlUser CR accounts):
   ALTER USER 'user'@'host' IDENTIFIED WITH caching_sha2_password RETAIN CURRENT PASSWORD;
   ```

4. Confirm replicas have caught up (for example `SHOW REPLICA STATUS`, or cluster node status `Replicating=True` and `Lagged=False`).
5. Change `spec.mysqlVersion` to the 8.4 target. The operator rolls out the new image (including `mysql-datadir-chown` when upgrading Percona 8.0→8.4).

## Related pages

- [Version profiles](mysql-version-profiles.md)
- [Operator configuration](operator-configuration.md)
- [MysqlCluster](mysql-cluster.md) — `UpgradeBlocked` condition
- [Troubleshooting](troubleshooting.md)
