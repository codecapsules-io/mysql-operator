# Monitoring

The operator and each MySQL cluster expose Prometheus-compatible metrics.

## Cluster pod metrics (mysqld_exporter)

Every MySQL pod runs a **mysqld_exporter** sidecar that exposes metrics on port **9125** at path `/metrics`.

Pod template annotations (set by the operator):

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "9125"
```

### Scrape with Prometheus Operator

Example `PodMonitor` for clusters in namespace `app`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: mysql-clusters
  namespace: monitoring
spec:
  namespaceSelector:
    matchNames:
      - app
  selector:
    matchLabels:
      app.kubernetes.io/managed-by: mysql.presslabs.org
  podMetricsEndpoints:
    - port: prometheus
      path: /metrics
      interval: 30s
```

Adjust the namespace selector and labels to match your deployment. MySQL pods label the metrics port as `prometheus`.

### Extra exporter flags

Pass additional mysqld_exporter arguments per cluster:

```yaml
spec:
  metricsExporterExtraArgs:
    - --collect.info_schema.processlist
```

See the [mysqld_exporter documentation](https://github.com/prometheus/mysqld_exporter) for available collectors.

## Operator metrics

The operator controller serves metrics on port **8080** (flag `--metrics-addr`, default `:8080`). The `mysql-operator` Service maps external port **9125** to this container port for scraping convenience.

Example `ServiceMonitor` for the operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mysql-operator
  namespace: monitoring
spec:
  namespaceSelector:
    matchNames:
      - mysql-operator
  selector:
    matchLabels:
      app.kubernetes.io/name: mysql-operator
  endpoints:
    - port: prometheus
      path: /metrics
      interval: 30s
```

Disable operator metrics with `--metrics-addr=0` in the StatefulSet args.

## Health probes

The operator container exposes `/healthz` and `/readyz` on port **8081** (`--healthz-addr`).

## Related pages

- [Operator configuration](operator-configuration.md) — `--metrics-exporter-image`, `--metrics-addr`
- [MysqlCluster](mysql-cluster.md) — `metricsExporterExtraArgs`, `podSpec.metricsExporterResources`
