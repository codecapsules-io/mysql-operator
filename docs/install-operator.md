<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Install the operator

This page describes how to install, verify, customize, and upgrade the operator using versioned Kubernetes manifests under [`deploy/manifests/`](https://github.com/codecapsules-io/mysql-operator/tree/master/deploy/manifests).

## Layout

Each release is stored under a semver directory named after the git tag:

```text
deploy/manifests/
  v0.6.3/          # Frozen legacy baseline (former Helm chart defaults)
  v0.7.0/          # Current release
    crds/          # CustomResourceDefinitions — apply first
    operator/      # RBAC, StatefulSet, Services, ConfigMaps — apply second
    kustomization.yaml
```

The `operator/` kustomization sets the namespace to `mysql-operator`.

## Fresh install

From a checkout of this repository (or a release archive that includes `deploy/manifests/`):

```shell
export OPERATOR_VERSION=v0.7.0
export OPERATOR_NAMESPACE=mysql-operator

kubectl create namespace "${OPERATOR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
```

One-step alternative (same result):

```shell
kubectl apply -k deploy/manifests/v0.7.0
```

### Verify

```shell
kubectl get crd | grep mysql.presslabs.org
kubectl get pods,svc -n mysql-operator
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

The operator runs as a StatefulSet named `mysql-operator` with two containers: the **operator** controller and **orchestrator**.

## Container images

Default manifests pin images to `ghcr.io/codecapsules-io/*` at the release tag, for example:

| Component | Example image |
|-----------|---------------|
| Operator | `ghcr.io/codecapsules-io/mysql-operator:v0.7.0` |
| Orchestrator | `ghcr.io/codecapsules-io/mysql-operator-orchestrator:v0.7.0` |
| Sidecar 5.7 | `ghcr.io/codecapsules-io/mysql-operator-sidecar-5.7:v0.7.0` |
| Sidecar 8.0 | `ghcr.io/codecapsules-io/mysql-operator-sidecar-8.0:v0.7.0` |
| Sidecar 8.4 | `ghcr.io/codecapsules-io/mysql-operator-sidecar-8.4:v0.7.0` |
| mysqld_exporter | `docker.io/prom/mysqld-exporter:v0.16.0` |

CI publishes operator images to **GitHub Container Registry** (`ghcr.io/codecapsules-io/...`). To use a different registry, edit image references in `deploy/manifests/<version>/operator/statefulset.yaml` before applying.

## Customizing your install

Edit files under `deploy/manifests/<version>/operator/` **before** applying. Common changes:

| File | What to change |
|------|----------------|
| `statefulset.yaml` | Image repositories/tags, operator `args`, replica count |
| `orchestrator-secret.yaml` | Orchestrator topology credentials (`TOPOLOGY_USER`, `TOPOLOGY_PASSWORD`) |
| `orchestrator-config.yaml` | Orchestrator JSON settings |
| `clusterrole.yaml` | RBAC if you fork with new API resources |

### Orchestrator topology password

The operator and Orchestrator use a shared topology user to connect to MySQL pods. Credentials live in the `mysql-operator-orc` secret (`orchestrator-secret.yaml`).

!!! warning "Keep topology credentials stable on upgrade"
    If you change the topology password in manifests without updating credentials already stored in cluster secrets, Orchestrator can lose communication with existing clusters. Set explicit credentials in `orchestrator-secret.yaml` and keep them across upgrades.

### Operator flags

Important defaults in `v0.7.0` `statefulset.yaml`:

- `--sidecar-mysql84-image` — required for Percona 8.4 clusters
- `--failover-before-shutdown=true` — triggers failover in the MySQL preStop hook
- `--orchestrator-uri=http://mysql-operator.mysql-operator/api`

For the full flag list, see [Operator configuration](operator-configuration.md).

## Upgrade an existing install

1. Apply the new version's CRDs: `kubectl apply -k deploy/manifests/<new-version>/crds`
2. Apply the new operator manifests: `kubectl apply -k deploy/manifests/<new-version>/operator`
3. Wait for rollout: `kubectl rollout status statefulset/mysql-operator -n mysql-operator`

Review [Operator upgrades](operator-upgrades.md) and [MySQL versions & upgrades](mysql-versions-and-upgrades.md) before crossing minor versions.

## Uninstall

```shell
export OPERATOR_VERSION=v0.7.0

kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/operator"
kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/crds"   # only when CRs are gone
```

## Maintainer: prepare manifests for a new release

See the maintainer section in [`deploy/manifests/README.md`](https://github.com/codecapsules-io/mysql-operator/blob/master/deploy/manifests/README.md) for `make deploy.crds`, copying the `operator/` tree, and validation steps.
