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

# Active maintenance

This document describes what **Code Capsules** actively maintains in this fork of the MySQL operator. It is published so users and contributors know what to expect from ongoing work on this repository.

This fork does **not** track [bitpoke/mysql-operator](https://github.com/bitpoke/mysql-operator). Maintenance scope, releases, and priorities are owned by Code Capsules.

## In scope

The following areas receive active maintenance:

| Area                               | Description                                                                                                                            |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| **New MySQL version integrations** | Adding and validating support for new MySQL / Percona Server versions (images, catalogs, profiles, and upgrade paths).                 |
| **Old MySQL version support**      | Keeping older supported MySQL versions operational (compatibility fixes, image updates, and regression fixes on legacy version lines). |
| **Core infrastructure bug fixes**  | Fixes to HA, failover, orchestration, reconciliation, and other core operator behavior required for reliable clusters.                 |
| **Ease-of-use features**           | New or improved APIs, documentation, and operator behavior that make clusters easier to deploy and operate.                            |

## Out of scope

The following are **not** part of active maintenance unless explicitly called out in a release or issue:

- **Helm charts** — From **0.7.0** onward, the charts under `deploy/charts/` are not actively maintained or tested. They may still work for some deployments, but chart values, templates, and install docs are best-effort only. Prefer the versioned manifests under [`deploy/manifests/`](deploy/manifests/) — see [`deploy/manifests/README.md`](deploy/manifests/README.md) for install and release preparation steps.
- **Built-in backup and restore** — The operator’s legacy backup subsystem (sidecar-driven backups, RClone integration, and related CR workflows) is not actively developed. Prefer external backup solutions or platform-level backup for new deployments.
- **Upstream Bitpoke parity** — We do not merge or chase upstream releases; do not assume feature parity with [bitpoke/mysql-operator](https://github.com/bitpoke/mysql-operator).
- **Commercial SLAs** — This repository is maintained for Code Capsules platform use; there is no published public support SLA.

## How to engage

- **Contributing code:** All changes go through pull requests. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for approval rules, CI expectations, and quality bar.
- **Bugs and features** in the in-scope areas above: open a GitHub issue with reproduction steps or a clear use case.
- **Security issues:** follow responsible disclosure practices for your organization; do not post credentials in public issues.
- **Roadmap:** Not published publicly; prioritization is handled internally by Code Capsules.

For version-specific behavior, see [`docs/mysql-version-upgrades.md`](docs/mysql-version-upgrades.md) and [`docs/mysql-version-profiles.md`](docs/mysql-version-profiles.md).
