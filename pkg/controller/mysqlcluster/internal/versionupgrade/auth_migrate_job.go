/*
Copyright 2026 Code Capsules

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package versionupgrade

import (
	"strconv"
	"strings"

	"github.com/blang/semver"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/apis/domain"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

const (
	mysqlAuthMigrateHost         = "MYSQL_AUTH_MIGRATE_HOST"
	mysqlAuthMigratePodHost      = "MYSQL_AUTH_MIGRATE_POD_HOST"
	mysqlAuthMigratePort         = "MYSQL_AUTH_MIGRATE_PORT"
	mysqlAuthMigrateTarget       = "MYSQL_AUTH_MIGRATE_TARGET_VERSION"
	mysqlAuthMigrateTargetPlugin = "MYSQL_AUTH_MIGRATE_TARGET_PLUGIN"
	mysqlAuthMigrateSidecarPort  = "MYSQL_AUTH_MIGRATE_SIDECAR_PORT"

	defaultAuthMigrateTargetPlugin = "caching_sha2_password"
)

func newAuthMigrateJob(cluster *mysqlcluster.MysqlCluster, target semver.Version) *batch.Job {
	labels := cluster.GetSelectorLabels()
	labels[domain.LabelJobType] = JobTypeAuthMigrate
	labels[domain.LabelCluster] = cluster.Name
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
		{Name: mysqlAuthMigratePodHost, Value: cluster.GetMasterHost()},
		{Name: mysqlAuthMigratePort, Value: strconv.Itoa(constants.MysqlPort)},
		{Name: mysqlAuthMigrateSidecarPort, Value: strconv.Itoa(constants.SidecarServerPort)},
		{Name: mysqlAuthMigrateTargetPlugin, Value: defaultAuthMigrateTargetPlugin},
		envVarFromOperatedSecret(cluster, "BACKUP_USER", "BACKUP_USER", false),
		envVarFromOperatedSecret(cluster, "BACKUP_PASSWORD", "BACKUP_PASSWORD", false),
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
// Migration runs as root via the master pod sidecar Unix socket (HTTP /auth-migrate), which can
// alter root@localhost and other SYSTEM_USER accounts. TCP root from a separate Job pod is unreliable
// when root@'%' is out of sync with spec.secretName ROOT_PASSWORD.
func authMigrateScript() string {
	return strings.TrimSpace(`
set -eu

POD_HOST="${MYSQL_AUTH_MIGRATE_POD_HOST:?}"
TARGET="${MYSQL_AUTH_MIGRATE_TARGET_VERSION:?}"
TARGET_PLUGIN="${MYSQL_AUTH_MIGRATE_TARGET_PLUGIN:-caching_sha2_password}"
SIDECAR_PORT="${MYSQL_AUTH_MIGRATE_SIDECAR_PORT:-8080}"
BACKUP_USER="${BACKUP_USER:?}"
BACKUP_PASS="${BACKUP_PASSWORD:?}"
AUTH_PATH="` + constants.SidecarAuthMigratePath + `"

log() { printf '%s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

require_bins() {
  command -v curl >/dev/null 2>&1 || die "curl not found in PATH"
}

wait_for_master_sidecar() {
  url="http://${POD_HOST}:${SIDECAR_PORT}${AUTH_PATH}"
  log "auth plugin migration (pre-rollout to ${TARGET}, target plugin ${TARGET_PLUGIN}) via master sidecar ${url}"
  i=1
  while [ "$i" -le 60 ]; do
    if curl -sf -o /dev/null -u "${BACKUP_USER}:${BACKUP_PASS}" \
      "http://${POD_HOST}:${SIDECAR_PORT}/health"; then
      return 0
    fi
    sleep 2
    i=$((i + 1))
  done
  die "master sidecar HTTP endpoint not reachable at ${POD_HOST}:${SIDECAR_PORT}"
}

run_migration() {
  url="http://${POD_HOST}:${SIDECAR_PORT}${AUTH_PATH}?target_plugin=${TARGET_PLUGIN}"
  log "requesting socket-based migration as root on ${POD_HOST}"
  resp="$(curl -sf -w '\n%{http_code}' -X POST -u "${BACKUP_USER}:${BACKUP_PASS}" "$url")"
  code="$(printf '%s' "$resp" | tail -n1)"
  body="$(printf '%s' "$resp" | sed '$d')"
  if [ "$code" != "200" ]; then
    die "auth migrate sidecar returned HTTP ${code}: ${body:-empty body}"
  fi
  log "${body:-auth plugin migration complete}"
}

require_bins
wait_for_master_sidecar
run_migration
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
