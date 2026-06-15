<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Versioned operator manifests

This directory stores **frozen Kubernetes manifests** for each operator release: CRDs and operator runtime resources (RBAC, StatefulSet, Services, ConfigMaps, and so on).

Use these manifests to install or upgrade the operator **without Helm**. Helm charts under `deploy/charts/` remain for reference but are not actively maintained from 0.7.0 onward (see [`MAINTENANCE.md`](../../MAINTENANCE.md)).

---

## Install the operator (cluster operators)

### Prerequisites

- A Kubernetes cluster with `kubectl` configured for it
- `kubectl` **1.14+** (built-in `kubectl apply -k` uses kustomize)
- Cluster admin permissions to install CRDs and cluster-scoped RBAC

Pick the operator release that matches the git tag you are deploying (directory name includes the `v` prefix, e.g. `v0.7.0`).

### Fresh install

From a checkout of this repository (or after extracting a release archive that includes `deploy/manifests/`):

```shell
export OPERATOR_VERSION=v0.7.0
export OPERATOR_NAMESPACE=mysql-operator

kubectl create namespace "${OPERATOR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
```

Apply order matters: **CRDs first**, then operator resources.

One-step alternative (same result; operator namespace is set to `mysql-operator` in kustomize):

```shell
kubectl apply -k "deploy/manifests/v0.7.0"
```

### Verify the install

```shell
kubectl get crd | grep mysql.presslabs.org
kubectl get pods,svc -n mysql-operator
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

The operator runs as a StatefulSet named `mysql-operator` in the `mysql-operator` namespace by default.

### Deploy a MySQL cluster

After the operator is running, create cluster resources in your **application namespace** (not necessarily `mysql-operator`). Example manifests are in [`examples/`](../../examples/):

```shell
kubectl apply -f examples/example-cluster-secret.yaml
kubectl apply -f examples/example-cluster.yaml
```

Adapt names, storage classes, and `mysqlVersion` before production use. See [`docs/mysql-version-upgrades.md`](../../docs/mysql-version-upgrades.md) for MySQL version and upgrade behavior.

### Upgrade an existing install

1. Apply the **new** version’s CRDs: `kubectl apply -k deploy/manifests/<new-version>/crds`
2. Apply the **new** operator manifests: `kubectl apply -k deploy/manifests/<new-version>/operator`
3. Wait for rollout: `kubectl rollout status statefulset/mysql-operator -n mysql-operator`

Review release notes and [`docs/mysql-version-upgrades.md`](../../docs/mysql-version-upgrades.md) before upgrading across minor versions.

### Uninstall

```shell
export OPERATOR_VERSION=v0.7.0

# Remove operator workload and RBAC (does not remove MysqlCluster CRs in other namespaces)
kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/operator"

# Remove CRDs only when no MysqlCluster / MysqlUser / MysqlDatabase / MysqlBackup resources remain
kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/crds"
```

Deleting CRDs removes all custom resources of those kinds cluster-wide. Ensure application namespaces are cleaned up first.

### Customizing your install

Default images and operator flags are defined in `deploy/manifests/<version>/operator/` (especially `statefulset.yaml`). To use private registries, different sidecar tags, or extra operator arguments, edit those YAML files **before** applying, or maintain a forked copy of the version directory.

---

## Layout

Each release is stored under a semver directory named after the git tag (including the `v` prefix):

```text
deploy/manifests/
  v0.7.0/
    crds/                  # CustomResourceDefinitions (apply first)
      kustomization.yaml
      mysql.presslabs.org_mysqlclusters.yaml
      ...
    operator/              # Controller RBAC + workload (apply second)
      kustomization.yaml
      serviceaccount.yaml
      orchestrator-secret.yaml
      orchestrator-config.yaml
      clusterrole.yaml
      clusterrolebinding.yaml
      orchestrator-raft-service.yaml
      service.yaml
      statefulset.yaml
    kustomization.yaml     # Applies crds + operator together
```

Each operator resource lives in its own file; kustomize assembles them via `operator/kustomization.yaml`.

---

## Prepare manifests for a new release (maintainers)

Follow these steps when cutting a new operator version (example: `v0.8.0` from `v0.7.0`).

### 1. Create the new version folder

Create an empty directory for the release:

```shell
export NEW_VERSION=v0.8.0
mkdir -p "deploy/manifests/${NEW_VERSION}"
```

### 2. Generate CRDs for the new version

From the repository root, regenerate CRDs from Go types and copy them into the new version directory:

```shell
make deploy.crds VERSION="${NEW_VERSION}"
```

This runs `kubebuilder.manifests` and writes only to `deploy/manifests/<version>/crds/`. It also creates a top-level `kustomization.yaml` if one does not exist yet.

Equivalent:

```shell
./hack/generate-deploy-manifests.sh "${NEW_VERSION}"
```

`make deploy.manifests` is an alias for `make deploy.crds`.

### 3. Copy operator manifests from the previous release

Copy the `operator/` tree from the last released version. Do **not** copy `crds/` — those were just generated in step 2.

```shell
export PREV_VERSION=v0.7.0
cp -R "deploy/manifests/${PREV_VERSION}/operator" "deploy/manifests/${NEW_VERSION}/"
```

**Shortcut:** Copy the entire previous version directory first (`cp -R deploy/manifests/v0.7.0 deploy/manifests/v0.8.0`), then run step 2 to refresh `crds/` in place. The operator files from the copy remain untouched.

### 4. Adjust manifests for the new release

Edit files under `deploy/manifests/${NEW_VERSION}/operator/`. At minimum, update:

| File                                       | What to change                                                                                        |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| `statefulset.yaml`                         | Operator, orchestrator, and sidecar **image tags**; operator **args** (e.g. new flags); replica count |
| `clusterrole.yaml`                         | **RBAC rules** if new API resources or verbs were added in code                                       |
| `orchestrator-config.yaml`                 | Orchestrator settings if defaults changed                                                             |
| `orchestrator-secret.yaml`                 | Topology credentials if you rotate them for this release                                              |
| All files with `app.kubernetes.io/version` | Set to the new version (e.g. `v0.8.0`)                                                                |

Search the operator directory for the previous version string to catch anything you missed:

```shell
grep -R "v0.7.0" "deploy/manifests/${NEW_VERSION}/operator/"
```

### 5. Validate before committing

```shell
kustomize build "deploy/manifests/${NEW_VERSION}/crds" > /dev/null
kustomize build "deploy/manifests/${NEW_VERSION}/operator" > /dev/null
kustomize build "deploy/manifests/${NEW_VERSION}" > /dev/null
```

Optionally apply to a test cluster:

```shell
kubectl apply -k "deploy/manifests/${NEW_VERSION}/crds"
kubectl apply -k "deploy/manifests/${NEW_VERSION}/operator"
```

### 6. Commit

Commit the full `deploy/manifests/${NEW_VERSION}/` tree as part of the release.

### Re-sync CRDs only (no new release)

If Go API types change on a branch but the operator manifests are unchanged:

```shell
make deploy.crds VERSION=v0.7.0
```

This overwrites `crds/` only and leaves `operator/` untouched.
