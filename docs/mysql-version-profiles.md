# MySQL version profiles (operator behavior)

Server **images** are resolved by `ImageResolver` (see [mysql-version-upgrades.md](mysql-version-upgrades.md)): `spec.image`, CLI overrides, catalog file, then built-in constants.

**Profiles** describe version-line behavior: replication SQL dialect, grant metadata, operator `my.cnf` fragments, sidecar logical key, and validation. Built-in Percona profiles are `percona-5.7`, `percona-8.0`, `percona-8.4`, `percona-9.7`, plus a last-resort `fallback-unknown` for unrecognized semvers.

## Declarative overlays

You can prepend profiles from a YAML file (merged **before** built-ins, first match wins):

```yaml
prependProfiles:
  - name: my-10x-line
    semverRange: ">=10.0.0 <11.0.0"
    baseProfile: percona-9.7
```

- **name**: Observability only (returned from `Profile.Name()` for custom entries).
- **semverRange**: A [blang/semver](https://github.com/blang/semver) range expression.
- **baseProfile**: One of `percona-5.7`, `percona-8.0`, `percona-8.4`, `percona-9.7`, or `fallback-unknown` to reuse that line’s replication, grants, sidecar key, and `my.cnf` defaults.

### Operator flags and Helm

- CLI: `--mysql-profile-overlay-file=/path/to/overlay.yaml`
- Helm: `mysqlVersionProfileOverlay.enabled`, `mountPath`, `fileName`, and `data` (see `values.yaml`). Mounts a ConfigMap and passes the flag to the operator container.

Sending **SIGHUP** to the operator process reloads the **profile overlay** file from disk and refreshes the in-process `ImageResolver` (CLI flags are not re-parsed; restart the operator to pick up catalog file or flag changes).

### Sidecar

The sidecar binary initializes the same registry from built-ins plus an optional file path in **`MYSQL_OPERATOR_PROFILE_OVERLAY_FILE`** (if you mount the same overlay into MySQL pods and set that env on the sidecar/init containers).

## Orchestrator and mysqld_exporter

Discovery and failover depend on the **Orchestrator** build in this repo’s Docker image (`images/mysql-operator-orchestrator/Dockerfile`), built from **[percona/orchestrator](https://github.com/percona/orchestrator)** at a pinned commit so topology discovery uses **MySQL 8.4+ replication SQL** (`SHOW REPLICA STATUS`, etc.). Older **openark/orchestrator** 3.2.x binaries hit parse errors on 8.4 (`near 'slave status'` / `near 'master status'`). Override `values.yaml` → `orchestrator.image` if you use a different build. The **mysqld_exporter** image defaults to `prom/mysqld-exporter:v0.16.0` via Helm `metricsExporter` and the operator’s `--metrics-exporter-image` default.
