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
  <a href="https://codecapsules.io/">
    <img alt="Code Capsules" title="Code Capsules" src="./logo.svg" width="400" style="color: black">
  </a>
</p>

<p align="center">
  <i>The simplest way to deploy your code.</i><br/>
  <a href="https://codecapsules.io/">https://codecapsules.io</a>
</p>

The **Code Capsules MySQL Operator** is a Kubernetes controller used to run and manage MySQL workloads for **Code Capsules** hosting. It deploys highly available MySQL clusters, handles backups, failover, and day‑to‑day operations needed to host MySQL-based capsules on our platform infrastructure.

This repository is a maintained fork of the open-source [mysql-operator](https://github.com/bitpoke/mysql-operator) project. **It does not track or adopt upstream changes from Bitpoke**; feature work, releases, and operational practices are owned by Code Capsules.

## Relationship to upstream

- **Upstream:** [bitpoke/mysql-operator](https://github.com/bitpoke/mysql-operator) (originally developed by Pressinfra SRL / Bitpoke).
- **This fork:** Independently maintained by Code Capsules for internal platform use. Do not assume compatibility with upstream charts, docs, or release cadence.

## Capabilities

The operator is intended to:

1. Deploy and operate MySQL clusters on Kubernetes (cluster-per-service model).
2. Provide HA, monitoring hooks, backups, and point-in-time recovery patterns suitable for managed hosting.
3. Support scheduled and on-demand backups and cluster cloning where configured.

For version-specific behavior (MySQL 8.4, upgrades, catalogs, profiles), see the documentation in [`docs/`](docs/).

## Documentation

| Topic                            | Location                                                                           |
| -------------------------------- | ---------------------------------------------------------------------------------- |
| MySQL version catalog & upgrades | [`docs/mysql-version-upgrades.md`](docs/mysql-version-upgrades.md)                 |
| Version profiles & overlays      | [`docs/mysql-version-profiles.md`](docs/mysql-version-profiles.md)                 |
| Helm chart values                | [`deploy/charts/mysql-operator/README.md`](deploy/charts/mysql-operator/README.md) |

## Deploying the controller

Install from the Helm chart in this repository (adjust namespace and values as required for your environment):

```shell
helm install mysql-operator ./deploy/charts/mysql-operator \
  --namespace mysql-operator --create-namespace
```

See the [chart README](deploy/charts/mysql-operator/README.md) for configuration options (sidecars, version catalog, profile overlays, images, and RBAC).

**Kubernetes version:** Confirm cluster compatibility with your target operator release before upgrading; see [`docs/mysql-version-upgrades.md`](docs/mysql-version-upgrades.md) for MySQL server upgrade behavior.

## Deploying a MySQL cluster

Example manifests are under [`examples/`](examples/). Apply a secret and cluster CR in your target namespace, for example:

```shell
kubectl apply -f examples/example-cluster-secret.yaml
kubectl apply -f examples/example-cluster.yaml
```

Adapt names, storage classes, and `mysqlVersion` to your environment before use in production.

## Roadmap and support

**Roadmap:** Not published publicly. Planning and prioritization are handled internally by Code Capsules.

**Support:** This operator is maintained for Code Capsules platform use. It is not affiliated with Bitpoke commercial support, sponsorship, or upstream issue SLAs.

## Tech notes

Clusters use **Percona Server for MySQL** (5.7 / 8.0 / 8.4 lines as configured) for backup tooling, monitoring, and operational features. Failover topology uses **Orchestrator** (see [`NOTICE`](NOTICE) for third-party attribution).

## License and legal notices

This project is licensed under the **Apache License, Version 2.0**.

### License text

You must comply with the full license terms. The complete license text is in the root [`LICENSE`](LICENSE) file:

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
