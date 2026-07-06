<!--
Copyright 2026 Code Capsules
SPDX-License-Identifier: Apache-2.0
-->

# Orchestrator

[Orchestrator](https://github.com/percona/orchestrator) manages MySQL replication topology and performs automatic failover. This operator bundles Orchestrator as a sidecar container in the `mysql-operator` StatefulSet.

## What Orchestrator does

- Discovers cluster members and their roles (primary, replica)
- Detects primary failure and promotes a replica
- Coordinates with the operator during planned shutdowns (`--failover-before-shutdown`)
- Surfaces topology in a web UI

Each `MysqlCluster` registers with Orchestrator using the alias `{cluster-name}.{namespace}`.

## Accessing the UI

Port-forward the operator Service (Orchestrator HTTP is exposed on port 80):

```shell
kubectl port-forward -n mysql-operator svc/mysql-operator 8080:80
```

Open `http://localhost:8080` in your browser.

The Service name is always `mysql-operator` in the `mysql-operator` namespace when using default manifests — unlike Helm releases, there is no release-name prefix.

## Cluster conditions

`MysqlCluster` status reflects Orchestrator state:

| Condition | Meaning |
|-----------|---------|
| `PendingFailoverAck` | A failover event needs acknowledgment in Orchestrator |
| `FailoverInProgress` | Orchestrator is actively failing over |

Check with:

```shell
kubectl describe mysql my-cluster
```

## MySQL 8.4 compatibility

This fork ships a custom Orchestrator image built from Percona Orchestrator with MySQL 8.4 replication SQL support. See [Version profiles](mysql-version-profiles.md).

## Topology credentials

Orchestrator connects to MySQL using the topology user configured in `mysql-operator-orc` and propagated to each cluster's operated secret. Keep credentials consistent across operator upgrades — see [Install the operator](install-operator.md).

## Related pages

- [Connecting applications](connecting-applications.md) — how failover affects the master Service
- [Troubleshooting](troubleshooting.md) — orchestrator registration and password issues
