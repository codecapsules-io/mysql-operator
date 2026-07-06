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

# Code Capsules MySQL Operator

<p align="center">
  <a href="https://codecapsules.io/?utm_source=github&utm_medium=referral&utm_campaign=mysql-operator">
    <img alt="Code Capsules" title="Code Capsules" src="./logo.svg" width="400" style="color: black">
  </a>
</p>

<p align="center">
  <i>The simplest way to deploy your code.</i><br/>
  <a href="https://codecapsules.io/?utm_source=github&utm_medium=referral&utm_campaign=mysql-operator">https://codecapsules.io</a>
</p>

The **Code Capsules MySQL Operator** is a Kubernetes controller used to run and manage MySQL workloads for **Code Capsules** hosting. It deploys highly available MySQL clusters, handles failover, and day-to-day operations needed to host MySQL-based capsules on our platform infrastructure.

This repository is a maintained fork of the open-source [mysql-operator](https://github.com/bitpoke/mysql-operator) project. **It does not track or adopt upstream changes from Bitpoke**; feature work, releases, and operational practices are owned by Code Capsules.

## Documentation

**Full documentation:** [https://codecapsules-io.github.io/mysql-operator/](https://codecapsules-io.github.io/mysql-operator/)

| Topic                     | Location                                                                                   |
| ------------------------- | ------------------------------------------------------------------------------------------ |
| Getting started & install | [Docs site](https://codecapsules-io.github.io/mysql-operator/getting-started/)             |
| `MysqlCluster` reference  | [Docs site](https://codecapsules-io.github.io/mysql-operator/mysql-cluster/)               |
| MySQL 8.4 & upgrades      | [Docs site](https://codecapsules-io.github.io/mysql-operator/mysql-versions-and-upgrades/) |
| Migrating from Helm       | [Docs site](https://codecapsules-io.github.io/mysql-operator/migrate-from-helm/)           |
| Active maintenance scope  | [`MAINTENANCE.md`](MAINTENANCE.md)                                                         |
| Contributing              | [`CONTRIBUTING.md`](CONTRIBUTING.md)                                                       |
| Manifest maintainer guide | [`deploy/manifests/README.md`](deploy/manifests/README.md)                                 |

### Running docs locally

The docs site is built with [MkDocs](https://www.mkdocs.org/) and the [Material theme](https://squidfunk.github.io/mkdocs-material/). Source lives in [`docs/`](docs/); navigation and site settings are in [`mkdocs.yml`](mkdocs.yml).

Requires Python 3 (3.12+ recommended). From the repository root:

```shell
python3 -m venv .venv-docs
source .venv-docs/bin/activate
pip install -r docs/requirements.txt
mkdocs serve
```

Open [http://127.0.0.1:8000](http://127.0.0.1:8000). `mkdocs serve` reloads when you edit files under `docs/`.

To validate the site the same way CI does:

```shell
mkdocs build --strict
```

Built output is written to `site/` (gitignored).

## Relationship to upstream

- **Upstream:** [bitpoke/mysql-operator](https://github.com/bitpoke/mysql-operator) (originally developed by Pressinfra SRL / Bitpoke).
- **This fork:** Independently maintained by Code Capsules for internal platform use. Do not assume compatibility with upstream Helm charts, docs, or release cadence.

## Capabilities

The operator is intended to:

1. Deploy and operate MySQL clusters on Kubernetes (cluster-per-service model).
2. Provide HA, monitoring hooks, and failover suitable for managed hosting.
3. Support Percona Server 5.7, 8.0, and 8.4 with operator-orchestrated upgrades.

Built-in backup and restore features exist but are **not actively maintained** for new deployments. See [Legacy backups](https://codecapsules-io.github.io/mysql-operator/legacy-backups/) on the docs site.

## Deploying the controller

Install and upgrade using versioned Kubernetes manifests under [`deploy/manifests/`](deploy/manifests/).

Quick start:

```shell
export OPERATOR_VERSION=v0.7.0
kubectl create namespace mysql-operator --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

See the [install guide](https://codecapsules-io.github.io/mysql-operator/install-operator/) for prerequisites, customization, and upgrades.

## Deploying a MySQL cluster

Example manifests are under [`examples/`](examples/). Apply a secret and cluster CR in your target namespace:

```shell
kubectl apply -f examples/example-cluster-secret.yaml
kubectl apply -f examples/example-cluster.yaml
```

Adapt names, storage classes, and `mysqlVersion` to your environment before use in production.

## Roadmap and support

**Active maintenance:** See [`MAINTENANCE.md`](MAINTENANCE.md) for in-scope and out-of-scope areas.

**Contributing:** See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the PR workflow and CI expectations.

**Roadmap:** Not published publicly. Planning is handled internally by Code Capsules.

**Support:** This operator is maintained for Code Capsules platform use. It is not affiliated with Bitpoke commercial support or upstream issue SLAs.

## Tech notes

Clusters use **Percona Server for MySQL** (5.7 / 8.0 / 8.4 lines as configured). Failover topology uses **Orchestrator** (see [`NOTICE`](NOTICE) for third-party attribution).

## License and legal notices

This project is licensed under the **Apache License, Version 2.0**.

### License text

The complete license text is in the root [`LICENSE`](LICENSE) file:

> http://www.apache.org/licenses/LICENSE-2.0

### NOTICE file (required for distributions)

Apache License 2.0 **Section 4(d)** requires that if the Work includes a `NOTICE` file, derivative works and distributions must include a **readable copy** of that notice. This repository includes [`NOTICE`](NOTICE). When you redistribute source or binaries (including container images built from this repo), **include the `NOTICE` file** alongside `LICENSE` in your distribution package or image documentation.

The [`NOTICE`](NOTICE) file summarizes:

- Copyright and attribution for the original **Pressinfra SRL** work.
- **Copyright 2026 Code Capsules** for modifications and additional contributions in this fork.
- Third-party components (for example **Percona Orchestrator** built into the orchestrator image).

### Copyright and attribution (Section 4(b)–(c))

When distributing or modifying this software:

1. **Retain** all copyright, patent, trademark, and attribution notices present in the source (including per-file headers in `pkg/`, `cmd/`, and other contributed files).
2. **You may add** your own copyright statement for your modifications, as permitted by the License.
3. **Do not** remove existing upstream notices (for example Pressinfra SRL, Platform9 Inc., The Kubernetes Authors, or Upbound Authors where present in individual files).

### Source form

Modified source files in this fork that contain substantive Code Capsules changes include a `Copyright 2026 Code Capsules` line in addition to upstream copyright lines, where applicable. New files authored for this fork are marked `Copyright 2026 Code Capsules` under the same Apache 2.0 terms.

### No additional restrictions

Apache 2.0 does not permit adding legal terms that contradict the License. This README does not change the License; it only describes how to comply with it when using or redistributing this project.

### Verification

Run the repository license header check:

```shell
./hack/license-check
```

---

**SPDX-License-Identifier:** Apache-2.0
