# Getting started

This guide walks you through installing the operator and deploying your first MySQL cluster.

## Prerequisites

You will need:

- A Kubernetes cluster with `kubectl` configured
- `kubectl` **1.14+** (built-in `kubectl apply -k` uses Kustomize)
- Cluster-admin permissions to install CRDs and cluster-scoped RBAC

Helm is **not** required. This fork installs the operator with `kubectl apply -k` against manifests in [`deploy/manifests/`](https://github.com/codecapsules-io/mysql-operator/tree/master/deploy/manifests).

## Install the operator

Pick the operator release that matches the git tag you are deploying (directory names include the `v` prefix, for example `v0.7.0`).

```shell
export OPERATOR_VERSION=v0.7.0
export OPERATOR_NAMESPACE=mysql-operator

kubectl create namespace "${OPERATOR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/crds"
kubectl apply -k "deploy/manifests/${OPERATOR_VERSION}/operator"
```

Apply order matters: **CRDs first**, then operator resources.

### Verify the install

```shell
kubectl get crd | grep mysql.presslabs.org
kubectl get pods,svc -n mysql-operator
kubectl rollout status statefulset/mysql-operator -n mysql-operator
```

You should see four CRDs (`mysqlclusters`, `mysqlusers`, `mysqldatabases`, `mysqlbackups`) and a running `mysql-operator` StatefulSet in the `mysql-operator` namespace.

For customization options (images, operator flags, orchestrator password), see [Install the operator](install-operator.md).

## Deploy your first cluster

Cluster resources live in your **application namespace**, not in `mysql-operator`.

### 1. Create a credentials secret

The cluster needs a secret with at least `ROOT_PASSWORD` (base64-encoded). Optional bootstrap fields `USER`, `PASSWORD`, and `DATABASE` create an application user and database on first start.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  ROOT_PASSWORD: bm90LXNvLXNlY3VyZQ==   # base64-encoded password
```

See [`examples/example-cluster-secret.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-cluster-secret.yaml) for a full example.

!!! note "Bootstrap-only fields"
    `USER`, `PASSWORD`, and `DATABASE` are used only when the cluster is first initialized. Changing them later does not update the MySQL server.

### 2. Create a MysqlCluster

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: my-cluster
spec:
  replicas: 2
  secretName: my-secret
  mysqlVersion: "8.0"
```

Apply both files:

```shell
kubectl apply -f examples/example-cluster-secret.yaml
kubectl apply -f examples/example-cluster.yaml
```

Adapt names, namespace, storage class, and `mysqlVersion` before production use. For Percona 8.4, set `mysqlVersion: "8.4"` — see [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

### 3. Check cluster status

```shell
kubectl get mysql
kubectl describe mysql my-cluster
```

When the cluster is ready, `status.conditions` includes `Ready=True` and `status.readyNodes` matches your replica count.

## Uninstall the operator

Removing the operator does **not** delete `MysqlCluster` resources in other namespaces.

```shell
export OPERATOR_VERSION=v0.7.0

kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/operator"
```

Delete CRDs only when no custom resources remain:

```shell
kubectl delete -k "deploy/manifests/${OPERATOR_VERSION}/crds"
```

!!! warning
    Deleting CRDs removes all `MysqlCluster`, `MysqlUser`, `MysqlDatabase`, and `MysqlBackup` objects cluster-wide.

## Next steps

- [MysqlCluster](mysql-cluster.md) — full configuration reference
- [Connecting applications](connecting-applications.md) — service endpoints and connection strings
- [Orchestrator](orchestrator.md) — failover UI and topology
- [Migrating from Helm](migrate-from-helm.md) — if you run the legacy Bitpoke Helm chart
