<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Migrating from Helm

Helm charts were **removed in v0.7.0**. If you currently run the operator via the upstream Bitpoke Helm chart (or an older fork release at **v0.6.3** or earlier), you can migrate to manifest-based installs.

## What changed

| Before (Helm) | After (this fork) |
|---------------|-------------------|
| `helm install` from `helm-charts.bitpoke.io` | `kubectl apply -k deploy/manifests/v0.7.0` |
| Chart `values.yaml` for images and flags | Edit YAML under `deploy/manifests/<version>/operator/` |
| `bitpoke/*` images | `codecapsules-io/*` images (by default) |
| No Percona 8.4 sidecar flag | `--sidecar-mysql84-image` on operator |

Frozen **v0.6.3** manifests under `deploy/manifests/v0.6.3/` mirror what the last Helm chart deployed. Use them as a reference when comparing your Helm values to the new layout.

## Typical migration path

### 1. Record your Helm values

Note image repositories/tags, `extraArgs`, orchestrator topology password, `watchNamespace`, and any custom sidecar or exporter images from your Helm release.

### 2. Compare with v0.6.3 manifests

Open `deploy/manifests/v0.6.3/operator/statefulset.yaml`. Defaults use `docker.io/bitpoke/*:v0.6.3` and `prom/mysqld-exporter:v0.13.0`.

### 3. Prepare v0.7.0 manifests

Before applying, edit `deploy/manifests/v0.7.0/operator/` if you need non-default:

- Image registries or tags
- Orchestrator topology credentials (set explicitly in `orchestrator-secret.yaml`)
- Operator CLI flags (for example `--namespace` to limit watch scope)
- `--sidecar-mysql84-image` if you run 8.4 clusters

### 4. Uninstall the Helm release

```shell
helm uninstall <release-name> -n mysql-operator
```

This removes the operator workload and RBAC created by Helm. It does **not** remove CRDs or `MysqlCluster` resources in application namespaces by default.

### 5. Apply v0.7.0 manifests

```shell
kubectl apply -k deploy/manifests/v0.7.0/crds
kubectl apply -k deploy/manifests/v0.7.0/operator
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

### 6. Verify

```shell
kubectl get crd | grep mysql.presslabs.org
kubectl get pods -n mysql-operator
kubectl get mysql -A
```

Existing `MysqlCluster` resources should reconcile under the new operator. Check `kubectl describe mysql <name>` for `Ready=True`.

## CRD changes

Applying `v0.7.0` CRDs updates API schemas in place. Review [CHANGELOG](https://github.com/codecapsules-io/mysql-operator/blob/master/CHANGELOG.md) for new fields (`sidecarImage`, `volumeSpec.keepAfterDelete`, `status.appliedMysqlVersion`).

## MySQL version upgrades after migration

Migration to v0.7.0 is an **operator** upgrade. MySQL **server** upgrades (for example 8.0 → 8.4) are separate — see [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Further reading

- [Install the operator](install-operator.md)
- [Operator upgrades](operator-upgrades.md)
