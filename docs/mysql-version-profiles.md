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

# MySQL version profiles (operator behavior)

Server **images** are resolved by `ImageResolver` (see [mysql-version-upgrades.md](mysql-version-upgrades.md)): `spec.image`, CLI overrides, catalog file, then built-in constants.

**Profiles** describe version-line behavior: replication SQL dialect, grant metadata, operator `my.cnf` fragments, sidecar logical key, and validation. Built-in Percona profiles are `percona-5.7`, `percona-8.0`, `percona-8.4` (MySQL 8.4+), plus a last-resort `fallback-unknown` for unrecognized semvers.

## Orchestrator and mysqld_exporter

Discovery and failover depend on the **Orchestrator** build in this repo’s Docker image (`images/mysql-operator-orchestrator/Dockerfile`), built from **[percona/orchestrator](https://github.com/percona/orchestrator)** at a pinned commit so topology discovery uses **MySQL 8.4+ replication SQL** (`SHOW REPLICA STATUS`, etc.). Older **openark/orchestrator** 3.2.x binaries hit parse errors on 8.4 (`near 'slave status'` / `near 'master status'`). Override the orchestrator container image in the operator deployment if you use a different build (see [`deploy/manifests/`](../deploy/manifests/README.md)). The **mysqld_exporter** image is set with `--metrics-exporter-image` (default `prom/mysqld-exporter:v0.16.0` in versioned manifests).
