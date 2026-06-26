<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Connecting applications

Each `MysqlCluster` exposes several Kubernetes Services for different connection patterns. You can wire your application Deployments, Jobs, or external tools to these endpoints.

## Service overview

For a cluster named `my-cluster` in namespace `app`:

| Service | DNS name | Purpose |
|---------|----------|---------|
| `{name}-mysql-master` | `my-cluster-mysql-master.app.svc` | **Writes** — targets the current primary |
| `{name}-mysql` | `my-cluster-mysql.app.svc` | **Reads** — all healthy nodes |
| `{name}-mysql-replicas` | `my-cluster-mysql-replicas.app.svc` | Healthy replicas **excluding** the primary |
| `mysql` (headless) | `my-cluster-mysql-0.mysql.app.svc` | Per-pod DNS for direct pod access |

All services expose MySQL on port **3306**.

### Write traffic

Point applications that insert, update, or delete data at the **master** service:

```text
my-cluster-mysql-master.app.svc.cluster.local:3306
```

The operator updates Service selectors after Orchestrator failover so the master service follows the current primary.

### Read traffic

For load-balanced reads across healthy replicas:

```text
my-cluster-mysql.app.svc.cluster.local:3306
```

Replicas with replication lag above `spec.maxSlaveLatency` are removed from this service.

### Direct pod access

StatefulSet pods are reachable via the headless service `mysql`:

```text
my-cluster-mysql-0.mysql.app.svc.cluster.local
my-cluster-mysql-1.mysql.app.svc.cluster.local
```

Use this for admin tasks or when you need a specific instance.

## Bootstrap credentials

Your application credentials come from the secret referenced by `spec.secretName` (for example `my-secret`):

| Key | Description |
|-----|-------------|
| `USER` | Application username (if set at bootstrap) |
| `PASSWORD` | Application password |
| `DATABASE` | Default database name |
| `ROOT_PASSWORD` | Root password (admin use) |

Construct a connection string from the master service and these values:

```text
mysql://USER:PASSWORD@my-cluster-mysql-master.app.svc.cluster.local:3306/DATABASE
```

For language-specific drivers, use the hostname, port, user, password, and database from the table above.

!!! note
    `USER`, `PASSWORD`, and `DATABASE` are applied only at cluster bootstrap. For ongoing user management, prefer [MysqlUser & MysqlDatabase](mysql-user-and-database.md).

## Operated secret (internal)

The operator creates `{cluster}-mysql-operated` with credentials for replication, Orchestrator, metrics, and backups. **Do not** use these for application traffic unless you understand their scope.

## Example: wiring a Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: app
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: DB_HOST
              value: my-cluster-mysql-master.app.svc.cluster.local
            - name: DB_PORT
              value: "3306"
          envFrom:
            - secretRef:
                name: my-secret
```

Map `USER`, `PASSWORD`, and `DATABASE` from the secret in your application code or add explicit `env` entries with `secretKeyRef`.

## Cross-namespace access

Services are namespace-scoped. Applications in another namespace should use the fully qualified DNS name:

```text
my-cluster-mysql-master.app.svc.cluster.local
```

Ensure network policies allow traffic from the client namespace.

## Related pages

- [Getting started](getting-started.md)
- [Orchestrator](orchestrator.md) — failover behavior affecting the master service
- [Monitoring](monitoring.md) — metrics endpoints on cluster pods
