<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# MysqlUser and MysqlDatabase

Beyond bootstrap credentials in the cluster secret, you can manage MySQL users and databases declaratively with `MysqlUser` and `MysqlDatabase` resources.

Both resources reference a `MysqlCluster` via `spec.clusterRef` (name and namespace). By default, the cluster must be in the **same namespace** unless the operator runs with `--allow-cross-namespace-user` or `--allow-cross-namespace-database`.

## MysqlUser

Creates a MySQL user with grants on specified schemas and tables.

### Example

Password secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-user-password
data:
  PASSWORD: bXlzcWwtcGFzc3dvcmQtZm9yLXVzZXI=
```

User resource:

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlUser
metadata:
  name: my-user
spec:
  user: user-name-in-mysql
  clusterRef:
    name: my-cluster
    namespace: default
  password:
    name: my-user-password
    key: PASSWORD
  allowedHosts:
    - localhost
    - "%"
  permissions:
    - schema: db-name-in-mysql
      tables: ["*"]
      permissions:
        - SELECT
        - INSERT
        - UPDATE
        - DELETE
```

Apply:

```shell
kubectl apply -f examples/example-user.yaml
```

### Deletion policy

By default, deleting the `MysqlUser` CR removes the MySQL user. To **retain** the user in MySQL:

```yaml
metadata:
  annotations:
    mysql-operator.presslabs.org/resourceDeletionPolicy: retain
```

### Status

When reconciled, the resource reports a `Ready` condition:

```shell
kubectl get mysqluser
kubectl describe mysqluser my-user
```

## MysqlDatabase

Creates a database on the referenced cluster.

```yaml
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlDatabase
metadata:
  name: my-database
spec:
  database: db-name-in-mysql
  clusterRef:
    name: my-cluster
    namespace: default
```

Optional fields `characterSet` and `collation` can be set in the full API schema.

### Deletion policy

Same annotation as `MysqlUser`:

```yaml
metadata:
  annotations:
    mysql-operator.presslabs.org/resourceDeletionPolicy: retain
```

## When to use which

| Approach | Best for |
|----------|----------|
| Bootstrap secret (`USER` / `PASSWORD` / `DATABASE`) | Initial app user at cluster creation |
| `MysqlUser` / `MysqlDatabase` | Ongoing declarative management, multiple apps, GitOps workflows |

## Auth plugin note (8.4)

Users created on MySQL 8.0 with `mysql_native_password` must be migrated before upgrading to 8.4. See [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## Related pages

- [MysqlCluster](mysql-cluster.md)
- [Operator configuration](operator-configuration.md) — cross-namespace flags
- [`examples/example-user.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-user.yaml)
- [`examples/example-database.yaml`](https://github.com/codecapsules-io/mysql-operator/blob/master/examples/example-database.yaml)
