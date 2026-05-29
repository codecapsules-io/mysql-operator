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

Version-specific SQL and `my.cnf` behavior is defined in built-in profiles and optional YAML overlays; see [mysql-version-profiles.md](mysql-version-profiles.md).

## Updating images without rebuilding the operator

1. Mount a ConfigMap (or Secret) into the operator pod at a path such as `/etc/mysql-operator/catalog/versions.properties`.
2. Pass `--mysql-version-catalog-file=/etc/mysql-operator/catalog/versions.properties`.
3. For **catalog** changes, restart the operator pod so `Validate()` re-reads the file. For **profile overlay** YAML, you may send **SIGHUP** instead (see [mysql-version-profiles.md](mysql-version-profiles.md)).

The catalog file is a list of lines `8.4.2=percona@sha256:...` (comments with `#` and blank lines are allowed).

## Helm

The chart can pass `--sidecar-mysql84-image`, optional catalog mounts, and optional profile overlay mounts via `values.yaml`. See `deploy/charts/mysql-operator/values.yaml`.

## MySQL server major upgrades

Changing `mysqlVersion` / `image` on an existing cluster is a **data plane** operation: follow Percona’s upgrade documentation, take backups, and roll instances in a safe order. The operator’s `mysql.presslabs.org/version` annotation refers to **operator** schema upgrades, not the MySQL server version.

### Pre-upgrade checks (operator)

When `spec.mysqlVersion` changes on a cluster that already has data on PVCs, the cluster controller:

1. Compares the desired version to `status.appliedMysqlVersion` (the version **fully running** on the data plane, not the spec alone).
2. Validates the upgrade path (no downgrades; one LTS line at a time, e.g. 8.0.x before 8.4.x).
3. For cross-line upgrades, runs a short-lived Job:
   - **Online** (cluster has running pods): sidecar connects to the master and runs `mysqlcheck --check` (does not mount the data PVC while mysqld is up).
   - **Offline** (no running pods): target server image opens the master PVC with `mysqld --upgrade=CHECK` (8.0.x) or `mysqld --upgrade=NONE` (8.4+; `CHECK` was removed in MySQL 8.4).
4. **Blocks** StatefulSet rollout until the `{cluster}-upgrade-check` Job succeeds (cross-line upgrades only).
5. Rolls out the new pod template (including any required init containers, e.g. `mysql-datadir-chown` for Percona 8.0→8.4).
6. Sets `status.appliedMysqlVersion` to match `spec.mysqlVersion` only after:
   - the StatefulSet template matches spec,
   - every replica is ready,
   - every required upgrade-check and auth-migrate Job has succeeded, and
   - **every init container on the current pod template has completed successfully on each pod**.

Patch-level bumps within the same profile line (e.g. `8.0.20` → `8.0.34`) skip the offline Job.

### Auth plugin migration (8.0 → 8.4+)

Percona/MySQL 8.4+ no longer loads `mysql_native_password`. Persistent accounts still using that plugin cannot authenticate after the upgrade.

Before the StatefulSet image changes, the operator runs a one-shot Job (`{cluster}-auth-migrate`) on the writable **master while it is still on the source line** (e.g. 8.0). The Job calls the master pod sidecar HTTP endpoint (`POST /auth-migrate`), which migrates accounts as **root over the local Unix socket** (from `spec.secretName` `ROOT_PASSWORD`). That path can alter **root@localhost** and other `SYSTEM_USER` rows; TCP `root` from a separate pod often fails when `root@'%'` is out of sync with the secret. The sidecar runs `ALTER USER … IDENTIFIED WITH <plugin>` (default `caching_sha2_password`, overridable via `MYSQL_AUTH_MIGRATE_TARGET_PLUGIN`) for **persistent** accounts still on `mysql_native_password`: **root** (all host rows), the optional cluster secret `USER` / `PASSWORD`, and any other non-system accounts that are not recreated by the sidecar `init-file`.

Operator utility users (`sys_operator`, `sys_replication`, `sys_exporter`, `sys_heartbeat`, orchestrator topology user) are `DROP USER` / `CREATE USER` on every mysqld start via `init-file`. On 8.4 the server default plugin is `caching_sha2_password` (the 8.0 `default-authentication-plugin=mysql_native_password` setting is not applied), so those accounts do **not** need pre-rollout migration. `MysqlUser` CR accounts and other app users without a known password in a secret use `RETAIN CURRENT PASSWORD` when migrated.

The Job is a **pre-rollout** step (with the datadir upgrade check): the image rollout is held until it succeeds. `status.appliedMysqlVersion` advances after rollout completes; auth migration does not block that separately.

Succeeded pre-rollout and post-rollout Jobs are deleted automatically once their phase completes (foreground cascade removes the Job pods too). Cluster annotations record completion so Jobs are not recreated on the next reconcile. Failed Jobs and their pods are left in place until the step succeeds so you can inspect logs.

If a cluster was marked applied on 8.4 before this Job existed, patch `status.appliedMysqlVersion` back to the prior 8.0 version (or run the `ALTER USER` statements manually) so the operator can run the migration Job and set applied again.

For a clean upgrade test: deploy on the source version (e.g. `8.0`), wait until `status.appliedMysqlVersion` matches and the cluster is Ready, then change `mysqlVersion` to the target (e.g. `8.4`).
