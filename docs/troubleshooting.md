# Troubleshooting

Common issues when running the Code Capsules MySQL operator.

## Cluster not Ready

**Symptoms:** `kubectl get mysql` shows `Ready=False` or `readyNodes` below `spec.replicas`.

**Checks:**

```shell
kubectl describe mysql <cluster-name>
kubectl get pods -l mysql.presslabs.org/cluster=<cluster-name>
kubectl logs <pod-name> -c mysql
```

Look at StatefulSet events, PVC binding, and MySQL container logs. Storage class misconfiguration is a frequent cause of pending PVCs.

## UpgradeBlocked condition

**Symptoms:** `status.conditions` includes `UpgradeBlocked=True` after changing `spec.mysqlVersion`.

**Cause:** Invalid upgrade path — downgrades and skipping LTS lines (for example 5.7 → 8.4 directly) are rejected.

**Fix:** Set `spec.mysqlVersion` to the next valid step (for example `8.0` before `8.4`). See [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## appliedMysqlVersion lags behind spec

**Symptoms:** `spec.mysqlVersion` is `8.4` but `status.appliedMysqlVersion` still shows `8.0.x`.

**Cause:** Rollout in progress, init container failure, or version check failing on one replica.

**Checks:**

```shell
kubectl get pods -l mysql.presslabs.org/cluster=<cluster-name>
kubectl describe pod <pod-name>   # init container status
```

Wait for all pods ready and init containers complete. For 8.0 → 8.4, complete the [auth plugin migration](mysql-versions-and-upgrades.md) first.

## Orchestrator not seeing the cluster

**Symptoms:** Failover does not work; Orchestrator UI empty or cluster shows errors.

**Checks:**

1. Cluster name not too long (Orchestrator registration limits).
2. Topology password consistent between `mysql-operator-orc` secret and cluster operated secrets.
3. Operator logs: `kubectl logs -n mysql-operator mysql-operator-0 -c operator`

After changing topology credentials in manifests, existing clusters may need credential alignment.

## Orchestrator password changed on operator upgrade

**Symptoms:** Orchestrator loses connection to MySQL pods after `kubectl apply` of new operator manifests.

**Cause:** Topology password in `orchestrator-secret.yaml` changed while cluster secrets still hold the old password.

**Fix:** Set explicit stable credentials in `orchestrator-secret.yaml` before the first production install. On upgrade, keep the same values or plan a coordinated rotation.

## Authentication failures after 8.4 upgrade

**Symptoms:** Application users cannot log in after upgrading to 8.4.

**Cause:** Accounts still using `mysql_native_password`.

**Fix:** Run the auth migration runbook on the primary **before** upgrading. See [MySQL versions & upgrades](mysql-versions-and-upgrades.md).

## PVCs left behind after cluster delete

**Symptoms:** PVCs remain when you expected them gone (or vice versa).

**Cause:** `spec.volumeSpec.keepAfterDelete` controls retention.

**Fix:** Set `keepAfterDelete: true` only when you intend to retain data. Manually delete orphaned PVCs named `data-<cluster>-mysql-*` when cleaning up.

## ROOT_PASSWORD not set

**Symptoms:** Cluster reconcile error mentioning `ROOT_PASSWORD not set in secret`.

**Fix:** Ensure `spec.secretName` points to a Secret with `ROOT_PASSWORD` in `data` (base64-encoded).

## Operator not reconciling clusters

**Checks:**

```shell
kubectl get pods -n mysql-operator
kubectl logs -n mysql-operator mysql-operator-0 -c operator
```

Verify CRDs are installed (`kubectl get crd mysqlclusters.mysql.presslabs.org`). If the operator runs with `--namespace`, it only watches that namespace.

## Getting help

For bugs in actively maintained areas, open an issue on [GitHub](https://github.com/codecapsules-io/mysql-operator/issues) with reproduction steps, operator version, `MysqlCluster` YAML (redacted secrets), and relevant logs.

See [maintenance scope](https://github.com/codecapsules-io/mysql-operator/blob/master/MAINTENANCE.md) for what is in scope.

## Related pages

- [Orchestrator](orchestrator.md)
- [Operator upgrades](operator-upgrades.md)
- [Install the operator](install-operator.md)
