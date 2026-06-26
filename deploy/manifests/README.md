<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Versioned operator manifests

This directory stores **frozen Kubernetes manifests** for each operator release: CRDs and operator runtime resources (RBAC, StatefulSet, Services, ConfigMaps, and so on).

## User documentation

Install, upgrade, customize, and migrate from Helm using the **[documentation site](https://codecapsules-io.github.io/mysql-operator/)**:

- [Install the operator](https://codecapsules-io.github.io/mysql-operator/install-operator/)
- [Getting started](https://codecapsules-io.github.io/mysql-operator/getting-started/)
- [Migrating from Helm](https://codecapsules-io.github.io/mysql-operator/migrate-from-helm/)
- [Operator upgrades](https://codecapsules-io.github.io/mysql-operator/operator-upgrades/)

Helm charts were removed in **0.7.0**. Frozen **v0.6.3** manifests remain for migration reference.

### Quick install

```shell
export OPERATOR_VERSION=v0.7.0
kubectl create namespace mysql-operator --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

---

## Layout

Each release is stored under a semver directory named after the git tag (including the `v` prefix):

```text
deploy/manifests/
  v0.6.3/                # Frozen legacy install (former Helm chart defaults; migration reference)
    crds/
    operator/
    kustomization.yaml
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

### Regenerating v0.6.3 manifests (legacy)

The `v0.6.3` tree is frozen for Helm migration reference and is not regenerated as part of normal releases. To inspect the original chart output, check out the `v0.6.3` git tag in the upstream or fork history.
