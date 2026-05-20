/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
	"github.com/bitpoke/mysql-operator/pkg/options"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
	"github.com/bitpoke/mysql-operator/pkg/util/mysqlversion"
)

const (
	dataVolumeName          = "data"
	mysqlUpgradeCheckHost   = "MYSQL_UPGRADE_CHECK_HOST"
	mysqlUpgradeCheckPort   = "MYSQL_UPGRADE_CHECK_PORT"
	mysqlUpgradeCheckTarget = "MYSQL_UPGRADE_CHECK_TARGET_VERSION"
)

func newUpgradeCheckJob(cluster *mysqlcluster.MysqlCluster, target semver.Version, _ *options.Options, sts *apps.StatefulSet) *batch.Job {
	online := ClusterHasRunningMySQL(cluster, sts)
	labels := cluster.GetSelectorLabels()
	labels["mysql.presslabs.org/job-type"] = JobTypeUpgradeCheck
	labels["mysql.presslabs.org/cluster"] = cluster.Name
	labels[upgradeCheckTargetLabel] = target.String()
	if online {
		labels["mysql.presslabs.org/upgrade-check-mode"] = "online"
	} else {
		labels["mysql.presslabs.org/upgrade-check-mode"] = "offline"
	}

	backoff := int32(0)

	var image string
	var args []string
	var volumeMounts []core.VolumeMount
	var volumes []core.Volume

	if online {
		image = cluster.GetSidecarImage()
		args = []string{onlineUpgradeCheckScript()}
	} else {
		image = cluster.GetMysqlImage()
		args = []string{offlineUpgradeCheckScript(target)}
		volumeMounts = []core.VolumeMount{{Name: dataVolumeName, MountPath: DataVolumeMountPath}}
		volumes = []core.Volume{{
			Name: dataVolumeName,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: MasterDataPVCName(cluster),
				},
			},
		}}
	}

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
		{Name: mysqlUpgradeCheckTarget, Value: target.String()},
	}
	if online {
		env = append(env,
			core.EnvVar{Name: mysqlUpgradeCheckHost, Value: cluster.GetMasterHost()},
			core.EnvVar{Name: mysqlUpgradeCheckPort, Value: strconv.Itoa(constants.MysqlPort)},
			envVarFromOperatedSecret(cluster, "OPERATOR_USER", "OPERATOR_USER", false),
			envVarFromOperatedSecret(cluster, "OPERATOR_PASSWORD", "OPERATOR_PASSWORD", false),
		)
	}

	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(cluster),
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
						Name:            JobContainerName,
						Image:           image,
						ImagePullPolicy: cluster.Spec.PodSpec.ImagePullPolicy,
						Command:         []string{"/bin/sh", "-ec"},
						Args:            args,
						Env:             env,
						SecurityContext: &core.SecurityContext{RunAsUser: &runAsUser},
						VolumeMounts:    volumeMounts,
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

func envVarFromOperatedSecret(cluster *mysqlcluster.MysqlCluster, name, key string, optional bool) core.EnvVar {
	return core.EnvVar{
		Name: name,
		ValueFrom: &core.EnvVarSource{
			SecretKeyRef: &core.SecretKeySelector{
				LocalObjectReference: core.LocalObjectReference{
					Name: cluster.GetNameForResource(mysqlcluster.Secret),
				},
				Key:      key,
				Optional: &optional,
			},
		},
	}
}

// onlineUpgradeCheckScript validates the live master before changing the StatefulSet image.
// MySQL 8.4+ removed --upgrade=CHECK; an online check avoids mounting the PVC while mysqld holds locks.
func onlineUpgradeCheckScript() string {
	return strings.TrimSpace(`
set -eu
host="${MYSQL_UPGRADE_CHECK_HOST:?}"
port="${MYSQL_UPGRADE_CHECK_PORT:-3306}"
user="${OPERATOR_USER:?}"
pass="${OPERATOR_PASSWORD:?}"
target="${MYSQL_UPGRADE_CHECK_TARGET_VERSION:?}"
echo "online MySQL upgrade check against ${host}:${port} (target ${target})"
ready=0
for i in $(seq 1 60); do
  if mysqladmin --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" ping 2>/dev/null; then
    ready=1
    break
  fi
  sleep 2
done
if [ "$ready" -ne 1 ]; then
  echo "master is not reachable for upgrade check"
  exit 1
fi
current="$(mysql --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" -NBe "SELECT VERSION()" 2>/dev/null || true)"
echo "current server version: ${current:-unknown}"
mysqlcheck --protocol=TCP -h "$host" -P "$port" -u "$user" -p"$pass" --all-databases --check
`)
}

// offlineUpgradeCheckScript opens the datadir with the target mysqld when no instance is running.
// MySQL 8.4+ does not support --upgrade=CHECK; use --upgrade=NONE (fail if upgrade required is OK).
func offlineUpgradeCheckScript(target semver.Version) string {
	useNONEOnly := "false"
	if mysqlversion.AtLeastMySQL84(target) {
		useNONEOnly = "true"
	}
	return strings.TrimSpace(fmt.Sprintf(`
set -eu
datadir="%s"
use_none_only=%s
if [ ! -d "$datadir/mysql" ] && [ ! -f "$datadir/ibdata1" ]; then
  echo "no MySQL datadir on volume; skipping offline upgrade check"
  exit 0
fi
run_offline_none() {
  "$1" --datadir="$datadir" --user=mysql --upgrade=NONE --skip-networking \
    --socket=/tmp/mysql-upgrade-check.sock --pid-file=/tmp/mysql-upgrade-check.pid 2>&1
}
run_offline_check() {
  "$1" --datadir="$datadir" --user=mysql --upgrade=CHECK \
    --skip-networking --socket=/tmp/mysql-upgrade-check.sock --pid-file=/tmp/mysql-upgrade-check.pid 2>&1
}
for mysqld in /usr/sbin/mysqld /usr/sbin/mysqld-debug; do
  [ -x "$mysqld" ] || continue
  if [ "$use_none_only" = "true" ]; then
    set +e
    out="$(run_offline_none "$mysqld")"
    code=$?
    set -e
    echo "$out"
    if [ "$code" -eq 0 ]; then
      exit 0
    fi
    if echo "$out" | grep -qiE 'upgrade.*required|must upgrade|needs upgrade|not upgrade'; then
      echo "datadir requires upgrade for target version (expected)"
      exit 0
    fi
    if echo "$out" | grep -qiE 'incompatible|cannot upgrade|not supported|corrupt'; then
      echo "datadir incompatible with target version"
      exit 1
    fi
    exit "$code"
  fi
  set +e
  out="$(run_offline_check "$mysqld")"
  code=$?
  set -e
  echo "$out"
  if [ "$code" -eq 0 ]; then
    exit 0
  fi
  if echo "$out" | grep -qiE "setting value 'CHECK' to 'upgrade'|unknown variable 'upgrade=CHECK'"; then
    echo "mysqld does not support --upgrade=CHECK, falling back to --upgrade=NONE"
    set +e
    out="$(run_offline_none "$mysqld")"
    code=$?
    set -e
    echo "$out"
    if [ "$code" -eq 0 ]; then
      exit 0
    fi
    if echo "$out" | grep -qiE 'upgrade.*required|must upgrade|needs upgrade|not upgrade'; then
      exit 0
    fi
    if echo "$out" | grep -qiE 'incompatible|cannot upgrade|not supported|corrupt'; then
      exit 1
    fi
    exit "$code"
  fi
  if echo "$out" | grep -qiE 'upgrade required|check identifies|not required'; then
    exit 0
  fi
  exit "$code"
done
echo "mysqld binary not found in target image"
exit 1
`, DataVolumeMountPath, useNONEOnly))
}

func jobFailed(job *batch.Job) (bool, string) {
	for _, c := range job.Status.Conditions {
		if c.Type == batch.JobFailed && c.Status == core.ConditionTrue {
			msg := c.Message
			if msg == "" {
				msg = c.Reason
			}
			return true, msg
		}
	}
	return false, ""
}

func jobSucceeded(job *batch.Job) bool {
	if job.Status.Succeeded > 0 {
		return true
	}
	for _, c := range job.Status.Conditions {
		if c.Type == batch.JobComplete && c.Status == core.ConditionTrue {
			return true
		}
	}
	return false
}

func jobFailureMessage(job *batch.Job) string {
	if ok, msg := jobFailed(job); ok {
		if msg != "" {
			return msg
		}
		return "upgrade check job failed"
	}
	if job.Status.Failed > 0 {
		return fmt.Sprintf("upgrade check job failed %d time(s)", job.Status.Failed)
	}
	return "upgrade check job did not succeed"
}
