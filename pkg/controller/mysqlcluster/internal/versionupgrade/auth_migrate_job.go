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
		{Name: mysqlAuthMigrateHost, Value: cluster.GetMasterServiceHost()},
		{Name: "MYSQL_AUTH_MIGRATE_POD_HOST", Value: cluster.GetMasterHost()},
		{Name: mysqlAuthMigratePort, Value: strconv.Itoa(constants.MysqlPort)},
		{Name: "MYSQL_ROOT_USER", Value: "root"},
		envVarFromClusterSecret(cluster, "MYSQL_ROOT_PASSWORD", "ROOT_PASSWORD", false),
		envVarFromOperatedSecret(cluster, "OPERATOR_USER", "OPERATOR_USER", false),
		envVarFromOperatedSecret(cluster, "OPERATOR_PASSWORD", "OPERATOR_PASSWORD", false),
		envVarFromOperatedSecret(cluster, "ORC_TOPOLOGY_USER", "ORC_TOPOLOGY_USER", true),
		envVarFromClusterSecret(cluster, "MYSQL_APP_USER", "USER", true),
		envVarFromClusterSecret(cluster, "MYSQL_APP_PASSWORD", "PASSWORD", true),
	}

	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthMigrateJobName(cluster),
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: batch.JobSpec{
			BackoffLimit: &backoff,
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

// authMigrateScript runs on the writable 8.0 primary before the STS image moves to 8.4+.
// MySQL 8.4+ does not load mysql_native_password. Operator utility users (sys_operator,
// sys_replication, …) are DROP/CREATE'd on every mysqld start via sidecar init-file and do
// not need pre-rollout migration. This job migrates persistent accounts: root, optional
// cluster secret USER, and any other non-init-file mysql_native_password users.
func authMigrateScript() string {
	return strings.TrimSpace(`
set -eu

HOST="${MYSQL_AUTH_MIGRATE_HOST:?}"
POD_HOST="${MYSQL_AUTH_MIGRATE_POD_HOST:-}"
PORT="${MYSQL_AUTH_MIGRATE_PORT:-3306}"
TARGET="${MYSQL_AUTH_MIGRATE_TARGET_VERSION:?}"
ROOT_USER="${MYSQL_ROOT_USER:-root}"
ROOT_PASS="${MYSQL_ROOT_PASSWORD:?}"
OP_USER="${OPERATOR_USER:?}"
OP_PASS="${OPERATOR_PASSWORD:?}"
ORC_USER="${ORC_TOPOLOGY_USER:-}"
APP_USER="${MYSQL_APP_USER:-}"
APP_PASS="${MYSQL_APP_PASSWORD:-}"

ALTER_SQL=""
trap 'rm -f "$ALTER_SQL"' EXIT

log() { printf '%s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# Plain TCP client flags (same as online upgrade-check). Do not use get-server-public-key here:
# on 8.0 with mysql_native_password it can break auth even when mysqladmin ping looks "alive".
mysql_cmd() {
  target_host="$1"
  login_user="$2"
  login_pass="$3"
  shift 3
  MYSQL_PWD="$login_pass" mysql --protocol=TCP -h "$target_host" -P "$PORT" -u "$login_user" "$@"
}

can_connect() {
  target_host="$1"
  login_user="$2"
  login_pass="$3"
  mysql_cmd "$target_host" "$login_user" "$login_pass" -e "SELECT 1" >/dev/null 2>&1
}

mysql_auth_error() {
  target_host="$1"
  login_user="$2"
  login_pass="$3"
  mysql_cmd "$target_host" "$login_user" "$login_pass" -e "SELECT 1" 2>&1 | head -1 | tr -d '\n'
}

require_bins() {
  command -v mysql >/dev/null 2>&1 || die "mysql client not found in PATH"
}

try_connect_any_host() {
  login_user="$1"
  login_pass="$2"
  shift 2
  for h in "$@"; do
    [ -z "$h" ] && continue
    if can_connect "$h" "$login_user" "$login_pass"; then
      HOST="$h"
      return 0
    fi
  done
  return 1
}

wait_for_master() {
  service_host="$HOST"
  log "auth plugin migration (pre-rollout to ${TARGET}); master service ${service_host}:${PORT}"
  i=1
  while [ "$i" -le 30 ]; do
    if try_connect_any_host "$ROOT_USER" "$ROOT_PASS" "$service_host" "$POD_HOST" \
      || try_connect_any_host "$OP_USER" "$OP_PASS" "$service_host" "$POD_HOST"; then
        log "authenticated via ${HOST}:${PORT}"
        return 0
    fi
    sleep 2
    i=$((i + 1))
  done
  root_err="$(mysql_auth_error "$service_host" "$ROOT_USER" "$ROOT_PASS")"
  op_err="$(mysql_auth_error "$service_host" "$OP_USER" "$OP_PASS")"
  die "cannot authenticate to master (tried ${service_host} and pod ${POD_HOST:-n/a}:${PORT}): ${ROOT_USER}: ${root_err:-unknown}; ${OP_USER}: ${op_err:-unknown}"
}

assert_server_pre_84() {
  ver="$(mysql_cmd "$HOST" "$1" "$2" -NBe "SELECT VERSION()")" || die "cannot read server version"
  log "server version: ${ver}"
  case "$ver" in
    8.[4-9]*|9.*)
      die "server is already on ${ver}; auth migration must complete before the 8.4 image rollout"
      ;;
  esac
}

assert_writable_primary() {
  i=1
  while [ "$i" -le 12 ]; do
    ro="$(mysql_cmd "$HOST" "$1" "$2" -NBe "SELECT @@GLOBAL.read_only OR @@GLOBAL.super_read_only" 2>/dev/null || echo 1)"
    if [ "$ro" = "0" ]; then
      return 0
    fi
    log "waiting for writable primary (${i}/12)"
    sleep 5
    i=$((i + 1))
  done
  die "primary is read-only; run auth migration on the writable master"
}

log_tcp_login_accounts() {
  log "MySQL matches user@host using the job pod client address; '%' allows TCP from this pod:"
  mysql_cmd "$HOST" "$1" "$2" -e "
    SELECT user, host, plugin
    FROM mysql.user
    WHERE user IN ('${ROOT_USER}', '${OP_USER}')
    ORDER BY user, host
  " 2>/dev/null || true
}

sql_escape_literal() {
  printf '%s' "$1" | sed "s/'/''/g"
}

mysql_quote_literal() {
  login_user="$1"
  login_pass="$2"
  value="$3"
  mysql_cmd "$HOST" "$login_user" "$login_pass" -NBe "SELECT QUOTE('$(sql_escape_literal "$value")')"
}

# Known passwords use BY (avoids ER 1820); others use RETAIN CURRENT PASSWORD (e.g. MysqlUser CRs).
password_for_user() {
  case "$1" in
    "${ROOT_USER}") printf '%s' "$ROOT_PASS" ;;
    "${APP_USER}") printf '%s' "$APP_PASS" ;;
  esac
}

# exclude_root=1 omits root (sys_operator cannot ALTER SYSTEM_USER accounts).
build_alter_file() {
  login_user="$1"
  login_pass="$2"
  exclude_root="${3:-0}"

  ALTER_SQL="$(mktemp)"
  users_list="$(mktemp)"
  root_filter=""
  if [ "$exclude_root" -eq 1 ]; then
    root_filter="AND user <> '${ROOT_USER}'"
  fi
  orc_exclude=""
  if [ -n "$ORC_USER" ]; then
    orc_exclude=", '${ORC_USER}'"
  fi

  mysql_cmd "$HOST" "$login_user" "$login_pass" -NBe "
    SELECT user, host
    FROM mysql.user
    WHERE plugin = 'mysql_native_password'
      AND user NOT IN ('mysql.infoschema', 'mysql.session', 'mysql.sys')
      AND user NOT IN ('${OP_USER}', 'sys_replication', 'sys_exporter', 'sys_heartbeat'${orc_exclude})
      ${root_filter}
    ORDER BY (user = '${ROOT_USER}'), user, host
  " >"$users_list" || die "failed to list accounts to migrate"

  if [ ! -s "$users_list" ]; then
    rm -f "$users_list"
    return 1
  fi

  while IFS="$(printf '\t')" read -r account_user account_host; do
    [ -z "$account_user" ] && continue
    pass="$(password_for_user "$account_user")"
    qu="$(mysql_quote_literal "$login_user" "$login_pass" "$account_user")"
    qh="$(mysql_quote_literal "$login_user" "$login_pass" "$account_host")"
    if [ -n "$pass" ]; then
      pq="$(mysql_quote_literal "$login_user" "$login_pass" "$pass")"
      printf 'ALTER USER %s@%s IDENTIFIED WITH caching_sha2_password BY %s;\n' "$qu" "$qh" "$pq" >>"$ALTER_SQL"
    else
      printf 'ALTER USER %s@%s IDENTIFIED WITH caching_sha2_password RETAIN CURRENT PASSWORD;\n' "$qu" "$qh" >>"$ALTER_SQL"
    fi
  done <"$users_list"
  rm -f "$users_list"

  if [ ! -s "$ALTER_SQL" ]; then
    return 1
  fi
  return 0
}

apply_alter_file() {
  login_user="$1"
  login_pass="$2"
  n="$(wc -l <"$ALTER_SQL" | tr -d ' ')"
  log "applying ${n} ALTER USER statement(s) as ${login_user} in one session:"
  sed 's/^/  /' "$ALTER_SQL"
  mysql_cmd "$HOST" "$login_user" "$login_pass" <"$ALTER_SQL"
}

count_native_root() {
  mysql_cmd "$HOST" "$1" "$2" -NBe "
    SELECT COUNT(*) FROM mysql.user
    WHERE user = '${ROOT_USER}' AND plugin = 'mysql_native_password'
  " 2>/dev/null || echo 0
}

require_bins
wait_for_master

USE_ROOT=0
if can_connect "$HOST" "$ROOT_USER" "$ROOT_PASS"; then
  USE_ROOT=1
  SESSION_USER="$ROOT_USER"
  SESSION_PASS="$ROOT_PASS"
elif can_connect "$HOST" "$OP_USER" "$OP_PASS"; then
  log "warning: cannot log in as ${ROOT_USER} over TCP; only non-${ROOT_USER} accounts can be migrated as ${OP_USER}"
  SESSION_USER="$OP_USER"
  SESSION_PASS="$OP_PASS"
else
  root_err="$(mysql_auth_error "$HOST" "$ROOT_USER" "$ROOT_PASS")"
  op_err="$(mysql_auth_error "$HOST" "$OP_USER" "$OP_PASS")"
  die "cannot authenticate over TCP as ${ROOT_USER} or ${OP_USER} at ${HOST}:${PORT} (${ROOT_USER}: ${root_err:-unknown}; ${OP_USER}: ${op_err:-unknown}); need ${ROOT_USER}@'%' or ${OP_USER}@'%' with passwords from spec.secretName / operated secret"
fi

assert_server_pre_84 "$SESSION_USER" "$SESSION_PASS"
assert_writable_primary "$SESSION_USER" "$SESSION_PASS"
log_tcp_login_accounts "$SESSION_USER" "$SESSION_PASS"

if [ "$USE_ROOT" -eq 1 ]; then
  if ! build_alter_file "$ROOT_USER" "$ROOT_PASS" 0; then
    log "no persistent mysql_native_password accounts to migrate (init-file users are recreated on 8.4 startup)"
    exit 0
  fi
  apply_alter_file "$ROOT_USER" "$ROOT_PASS"
  log "auth plugin migration complete"
  exit 0
fi

if ! build_alter_file "$OP_USER" "$OP_PASS" 1; then
  if [ "$(count_native_root "$OP_USER" "$OP_PASS")" = "0" ]; then
    log "no persistent mysql_native_password accounts to migrate (init-file users are recreated on 8.4 startup)"
    exit 0
  fi
  die "${ROOT_USER} still uses mysql_native_password but ${ROOT_USER} TCP login failed; migrate ${ROOT_USER} (all host rows) as root via socket on the master pod, delete the auth-migrate job, and retry"
fi

apply_alter_file "$OP_USER" "$OP_PASS"
if [ "$(count_native_root "$OP_USER" "$OP_PASS")" != "0" ]; then
  die "${ROOT_USER} still uses mysql_native_password; migrate ${ROOT_USER} (all host rows) via socket on the master pod, delete the auth-migrate job, and retry"
fi
log "auth plugin migration complete (non-root persistent accounts; init-file users will be recreated on 8.4 startup)"
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
