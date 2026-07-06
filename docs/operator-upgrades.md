<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Operator upgrades

This page covers upgrading the **operator controller** (the software in `mysql-operator` namespace), not MySQL server version changes on your clusters.

## Semantic versioning impact

| Release type                            | Typical impact                                                                |
| --------------------------------------- | ----------------------------------------------------------------------------- |
| **Patch** (for example v0.7.0 → v0.7.1) | Bug fixes; operator pod may restart; clusters usually unaffected              |
| **Minor** (for example v0.7.x → v0.8.0) | New features; operator restarts; review release notes for CRD or flag changes |
| **Major**                               | Breaking changes; may require manual steps                                    |

MySQL **server** upgrades are documented separately in [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Upgrade procedure

### Step 1 — Review the changelog

Read the [CHANGELOG](https://github.com/codecapsules-io/mysql-operator/blob/master/CHANGELOG.md) for your target version.

### Step 2 — Apply new CRDs

```shell
kubectl apply -k deploy/manifests/<new-version>/crds
```

### Step 3 — Apply new operator manifests

```shell
kubectl apply -k deploy/manifests/<new-version>/operator
```

### Step 4 — Wait for rollout

```shell
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

### Step 5 — Verify existing clusters

```shell
kubectl get mysql -A
kubectl describe mysql <name>
```

## v0.7.0 notes (from v0.6.x / Helm)

Key changes when moving to this fork's **v0.7.0**:

| Area            | Change                                                                                                           |
| --------------- | ---------------------------------------------------------------------------------------------------------------- |
| Packaging       | Helm charts removed; use `deploy/manifests/v0.7.0`                                                               |
| Images          | `ghcr.io/codecapsules-io/*` instead of `docker.io/bitpoke/*`                                                     |
| MySQL 8.4       | New `--sidecar-mysql84-image` flag required for 8.4 clusters                                                     |
| mysqld_exporter | Updated to v0.16.0                                                                                               |
| API             | `spec.sidecarImage`, `spec.volumeSpec.keepAfterDelete`, `status.appliedMysqlVersion`, `UpgradeBlocked` condition |
| Upgrades        | Operator-orchestrated MySQL LTS upgrades with path validation                                                    |

See [Migrating from Helm](migrate-from-helm.md) for the Helm-specific path.

## v0.6.3 baseline

Frozen manifests at `deploy/manifests/v0.6.3/` match the last upstream Helm chart layout. They are a **reference for migration**, not maintained for new features.

## Orchestrator password on upgrade

When upgrading operator manifests, preserve orchestrator topology credentials unless you intend to rotate them cluster-wide. Changing `TOPOLOGY_PASSWORD` in `orchestrator-secret.yaml` without updating operated secrets on existing clusters breaks Orchestrator discovery.

## CRD updates

`kubectl apply -k deploy/manifests/<version>/crds` updates CRD schemas in place. Existing custom resources remain; new optional fields become available.

Delete CRDs only during a full uninstall when no custom resources remain.

## Related pages

- [Install the operator](install-operator.md)
- [MySQL versions & upgrades](mysql-versions-and-upgrades.md)
- [Troubleshooting](troubleshooting.md)
