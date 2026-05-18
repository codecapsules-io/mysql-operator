# MySQL version catalog and upgrades

This operator resolves the Percona (or other) **server image** for a `MysqlCluster` in this order:

1. `spec.image` on the cluster (highest priority).
2. `--mysql-versions-to-image` CLI entries for the exact semver string (e.g. `9.7.0=...`).
3. Lines from `--mysql-version-catalog-file` (e.g. a ConfigMap mounted as a file), same `semver=image` format.
4. Built-in defaults in `pkg/util/constants/constants.go`.

**Sidecar images** are chosen by resolving the server semver to a **profile** `SidecarProfileKey` (`percona-57`, `percona-80`, `percona-84`, `percona-97`) and mapping that to operator flags (`--sidecar-image`, `--sidecar-mysql8-image`, optional `--sidecar-mysql84-image`, `--sidecar-mysql97-image`). You can override with `spec.sidecarImage` on a cluster.

Version-specific SQL and `my.cnf` behavior is defined in built-in profiles and optional YAML overlays; see [mysql-version-profiles.md](mysql-version-profiles.md).

## Updating images without rebuilding the operator

1. Mount a ConfigMap (or Secret) into the operator pod at a path such as `/etc/mysql-operator/catalog/versions.properties`.
2. Pass `--mysql-version-catalog-file=/etc/mysql-operator/catalog/versions.properties`.
3. For **catalog** changes, restart the operator pod so `Validate()` re-reads the file. For **profile overlay** YAML, you may send **SIGHUP** instead (see [mysql-version-profiles.md](mysql-version-profiles.md)).

The catalog file is a list of lines `8.4.2=percona@sha256:...` (comments with `#` and blank lines are allowed).

## Helm

The chart can pass `--sidecar-mysql84-image`, `--sidecar-mysql97-image`, optional catalog mounts, and optional profile overlay mounts via `values.yaml`. See `deploy/charts/mysql-operator/values.yaml`.

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

Percona/MySQL 8.4+ no longer loads `mysql_native_password`. Accounts created on the 8.0 line (root, application `USER`, and any other legacy users) must use `caching_sha2_password` instead.

After the StatefulSet rolls out the target image and pods are ready, the operator runs a one-shot Job (`{cluster}-auth-migrate`) that connects to the **master** as `sys_operator` and runs `ALTER USER … IDENTIFIED WITH caching_sha2_password` for every non-system account still on `mysql_native_password` (passwords are retained). Operator-managed utility users are already recreated on each start via `init-file`; this Job covers **root**, the secret application user, and any other remaining accounts.

`status.appliedMysqlVersion` is not advanced to 8.4 until this Job succeeds. Once `appliedMysqlVersion` matches `8.4`, the auth migration for that upgrade is complete—no separate annotation is stored.

If a cluster was marked applied on 8.4 before this Job existed, patch `status.appliedMysqlVersion` back to the prior 8.0 version (or run the `ALTER USER` statements manually) so the operator can run the migration Job and set applied again.

For a clean upgrade test: deploy on the source version (e.g. `8.0`), wait until `status.appliedMysqlVersion` matches and the cluster is Ready, then change `mysqlVersion` to the target (e.g. `8.4`).
