/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"strconv"
	"strings"

	"github.com/blang/semver"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

const (
	mysqlAuthMigrateHost   = "MYSQL_AUTH_MIGRATE_HOST"
	mysqlAuthMigratePort   = "MYSQL_AUTH_MIGRATE_PORT"
	mysqlAuthMigrateTarget = "MYSQL_AUTH_MIGRATE_TARGET_VERSION"
)

func newAuthMigrateJob(cluster *mysqlcluster.MysqlCluster, target semver.Version) *batch.Job {
	labels := cluster.GetSelectorLabels()
	labels["mysql.presslabs.org/job-type"] = JobTypeAuthMigrate
	labels["mysql.presslabs.org/cluster"] = cluster.Name
	labels[authMigrateTargetLabel] = target.String()

	backoff := int32(0)
	ttl := int32(600)

	h := mysqlversioning.ProfileFor(target).PodSecurityHints(cluster.IsPerconaImage())
	runAsUser := int64(999)
	if h.MysqlRunAsUser != nil {
		runAsUser = *h.MysqlRunAsUser
	}
	fsGroup := int64(999)
	if h.FSGroup != 0 {
		fsGroup = h.FSGroup
	}

	env := []core.EnvVar{
		{Name: mysqlAuthMigrateTarget, Value: target.String()},
		{Name: mysqlAuthMigrateHost, Value: cluster.GetMasterHost()},
		{Name: mysqlAuthMigratePort, Value: strconv.Itoa(constants.MysqlPort)},
		envVarFromOperatedSecret(cluster, "OPERATOR_USER", "OPERATOR_USER", false),
		envVarFromOperatedSecret(cluster, "OPERATOR_PASSWORD", "OPERATOR_PASSWORD", false),
	}

	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthMigrateJobName(cluster),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: batch.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					RestartPolicy:      core.RestartPolicyNever,
					SecurityContext:    &core.PodSecurityContext{FSGroup: &fsGroup},
					ImagePullSecrets:   cluster.Spec.PodSpec.ImagePullSecrets,
					ServiceAccountName: cluster.Spec.PodSpec.ServiceAccountName,
					NodeSelector:       cluster.Spec.PodSpec.NodeSelector,
					Tolerations:        cluster.Spec.PodSpec.Tolerations,
					Affinity:           cluster.Spec.PodSpec.Affinity,
					Containers: []core.Container{{
						Name:            AuthMigrateJobContainerName,
						Image:           cluster.GetSidecarImage(),
						ImagePullPolicy: cluster.Spec.PodSpec.ImagePullPolicy,
						Command:         []string{"/bin/sh", "-ec"},
						Args:            []string{authMigrateScript()},
						Env:             env,
						SecurityContext: &core.SecurityContext{RunAsUser: &runAsUser},
					}},
				},
			},
		},
	}
}

// authMigrateScript connects to the live master and rewrites every mysql_native_password account
// (except built-in system users) to caching_sha2_password, retaining existing passwords.
func authMigrateScript() string {
	return strings.TrimSpace(`
set -eu
host="${MYSQL_AUTH_MIGRATE_HOST:?}"
port="${MYSQL_AUTH_MIGRATE_PORT:-3306}"
user="${OPERATOR_USER:?}"
pass="${OPERATOR_PASSWORD:?}"
target="${MYSQL_AUTH_MIGRATE_TARGET_VERSION:?}"
# Debian default-mysql-client is often MariaDB's mysql, which does not support
# --get-server-public-key (Oracle MySQL 8.0+ only). Use it when the binary advertises it.
mysql_cli() {
  if mysql --help 2>/dev/null | grep -qF get-server-public-key; then
    mysql --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" --get-server-public-key "$@"
  else
    mysql --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" "$@"
  fi
}
for bin in mysql mysqladmin; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "required client ${bin} not found in PATH (sidecar image must include default-mysql-client)"
    exit 1
  fi
done
echo "MySQL auth plugin migration against ${host}:${port} (target ${target})"
ready=0
for i in $(seq 1 60); do
  if mysqladmin --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" ping 2>/dev/null; then
    ready=1
    break
  fi
  sleep 2
done
if [ "$ready" -ne 1 ]; then
  echo "master is not reachable for auth plugin migration"
  exit 1
fi
version_err="$(mktemp)"
current="$(mysql_cli -NBe "SELECT VERSION()" 2>"$version_err" || true)"
if [ -z "$current" ]; then
  echo "current server version: unknown"
  if [ -s "$version_err" ]; then
    echo "mysql client error:"
    cat "$version_err"
  else
    echo "mysql client returned no version (check OPERATOR_USER auth and master host)"
  fi
  rm -f "$version_err"
  exit 1
fi
rm -f "$version_err"
echo "current server version: ${current}"
case "${current}" in
  8.[4-9]*|9.*) ;;
  *)
    echo "master is not on MySQL 8.4+ yet; waiting for version rollout"
    exit 1
    ;;
esac
rows="$(mysql_cli -NBe "
  SELECT user, host
  FROM mysql.user
  WHERE plugin = 'mysql_native_password'
    AND user NOT IN ('mysql.infoschema', 'mysql.session', 'mysql.sys')
" 2>/dev/null || true)"
if [ -z "$rows" ]; then
  echo "no mysql_native_password accounts to migrate"
  exit 0
fi
count=0
printf '%s\n' "$rows" | while IFS=$'\t' read -r u h; do
  [ -z "$u" ] && continue
  u_esc="$(printf '%s' "$u" | sed "s/'/''/g")"
  h_esc="$(printf '%s' "$h" | sed "s/'/''/g")"
  acct="'${u_esc}'@'${h_esc}'"
  echo "migrating ${acct} to caching_sha2_password"
  mysql_cli -e "ALTER USER ${acct} IDENTIFIED WITH caching_sha2_password"
  count=$((count + 1))
done
echo "auth plugin migration finished"
`)
}

func authMigrateJobFailureMessage(job *batch.Job) string {
	if ok, msg := jobFailed(job); ok {
		if msg != "" {
			return msg
		}
		return "auth plugin migration job failed"
	}
	if job.Status.Failed > 0 {
		return "auth plugin migration job failed"
	}
	return "auth plugin migration job did not succeed"
}
