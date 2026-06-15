<!--
Copyright 2026 Code Capsules

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# MySQL version catalog and upgrades

This operator resolves the Percona (or other) **server image** for a `MysqlCluster` in this order:

1. `spec.image` on the cluster (highest priority).
2. `--mysql-versions-to-image` CLI entries for the exact semver string (e.g. `8.4.0=...`).
3. Lines from `--mysql-version-catalog-file` (e.g. a ConfigMap mounted as a file), same `semver=image` format.
4. Built-in defaults in `pkg/util/constants/constants.go`.

**Sidecar images** are chosen by resolving the server semver to a **profile** `SidecarProfileKey` (`percona-57`, `percona-80`, `percona-84`) and mapping that to operator flags (`--sidecar-image`, `--sidecar-mysql8-image`, optional `--sidecar-mysql84-image`). You can override with `spec.sidecarImage` on a cluster.

Version-specific SQL and `my.cnf` behavior is defined in built-in profiles; see [mysql-version-profiles.md](mysql-version-profiles.md).

## Updating images without rebuilding the operator

1. Mount a ConfigMap (or Secret) into the operator pod at a path such as `/etc/mysql-operator/catalog/versions.properties`.
2. Pass `--mysql-version-catalog-file=/etc/mysql-operator/catalog/versions.properties`.
3. Restart the operator pod so `Validate()` re-reads the catalog file.

The catalog file is a list of lines `8.4.2=percona@sha256:...` (comments with `#` and blank lines are allowed).

## Helm

> From **0.7.0** onward, Helm charts are not actively supported (see [`MAINTENANCE.md`](../MAINTENANCE.md)). The notes below apply to the legacy chart only.

The chart can pass `--sidecar-mysql84-image` and optional catalog mounts via `values.yaml`. See `deploy/charts/mysql-operator/values.yaml`.

## MySQL server major upgrades

Changing `mysqlVersion` / `image` on an existing cluster is a **data plane** operation: follow Percona’s upgrade documentation, take backups, and roll instances in a safe order. The operator’s `mysql.presslabs.org/version` annotation refers to **operator** schema upgrades, not the MySQL server version.

### Upgrade orchestration (operator)

When `spec.mysqlVersion` changes on a cluster that already has data on PVCs, the cluster controller:

1. Compares the desired version to `status.appliedMysqlVersion` (the version **fully running** on the data plane, not the spec alone).
2. Validates the upgrade path (no downgrades; one LTS line at a time, e.g. 8.0.x before 8.4.x). Invalid paths set the `UpgradeBlocked` condition and keep the cluster on its current version until `spec.mysqlVersion` is corrected.
3. Rolls out the new pod template for valid paths (including any required init containers, e.g. `mysql-datadir-chown` for Percona 8.0→8.4).
4. Sets `status.appliedMysqlVersion` to match `spec.mysqlVersion` only after:
   - the StatefulSet template matches spec,
   - every replica is ready, and
   - **every init container on the current pod template has completed successfully on each pod**.

Patch-level bumps within the same profile line (e.g. `8.0.20` → `8.0.34`) follow the same rollout and completion gates without extra steps.

The operator does **not** run pre-upgrade compatibility Jobs (such as `mysqlsh util.checkForServerUpgrade`). Run Percona’s upgrade checks and take backups yourself before changing `spec.mysqlVersion`.

For a clean upgrade test: deploy on the source version (e.g. `8.0`), wait until `status.appliedMysqlVersion` matches and the cluster is Ready, then change `mysqlVersion` to the target (e.g. `8.4`).

### Auth plugin migration (8.0 → 8.4+, manual prerequisite)

The operator **does not** migrate authentication plugins. Percona/MySQL 8.4+ no longer loads `mysql_native_password`; persistent accounts still using that plugin cannot authenticate after the upgrade.

**Complete this step on the writable primary before changing `spec.mysqlVersion` to 8.4.x.**

Operator utility users (`sys_operator`, `sys_replication`, `sys_exporter`, `sys_heartbeat`, orchestrator topology user) are `DROP USER` / `CREATE USER` on every mysqld start via `init-file`. On 8.4 the server default plugin is `caching_sha2_password`, so those accounts do **not** need manual migration.

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

   -- When the password is unknown (e.g. MysqlUser CR accounts):
   ALTER USER 'user'@'host' IDENTIFIED WITH caching_sha2_password RETAIN CURRENT PASSWORD;
   ```

4. Confirm replicas have caught up (e.g. `SHOW REPLICA STATUS`, or cluster node status `Replicating=True` and `Lagged=False`).
5. Change `spec.mysqlVersion` to the 8.4 target. The operator rolls out the new image (including `mysql-datadir-chown` when upgrading Percona 8.0→8.4).
