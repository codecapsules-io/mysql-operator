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
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/apis/domain"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

const (
	mysqlUpgradeCheckHost   = "MYSQL_UPGRADE_CHECK_HOST"
	mysqlUpgradeCheckPort   = "MYSQL_UPGRADE_CHECK_PORT"
	mysqlUpgradeCheckTarget = "MYSQL_UPGRADE_CHECK_TARGET_VERSION"
)

func newUpgradeCheckJob(cluster *mysqlcluster.MysqlCluster, target semver.Version, _ *options.Options) (*batch.Job, error) {
	labels := cluster.GetSelectorLabels()
	labels[domain.LabelJobType] = JobTypeUpgradeCheck
	labels[domain.LabelCluster] = cluster.Name
	labels[upgradeCheckTargetLabel] = target.String()
	labels[domain.LabelUpgradeCheckMode] = domain.UpgradeCheckModeOnline

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

	host := cluster.GetMasterHost()
	if isMultiReplica(cluster) {
		host = cluster.GetMasterServiceHost()
	}

	env := []core.EnvVar{
		{Name: mysqlUpgradeCheckTarget, Value: target.String()},
		{Name: mysqlUpgradeCheckHost, Value: host},
		{Name: mysqlUpgradeCheckPort, Value: strconv.Itoa(constants.MysqlPort)},
		envVarFromOperatedSecret(cluster, "OPERATOR_USER", "OPERATOR_USER", false),
		envVarFromOperatedSecret(cluster, "OPERATOR_PASSWORD", "OPERATOR_PASSWORD", false),
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
						Image:           cluster.GetSidecarImage(),
						ImagePullPolicy: cluster.Spec.PodSpec.ImagePullPolicy,
						Command:         []string{"/bin/sh", "-ec"},
						Args:            []string{upgradeCheckScript()},
						Env:             env,
						SecurityContext: &core.SecurityContext{RunAsUser: &runAsUser},
					}},
				},
			},
		},
	}, nil
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

// upgradeCheckScript validates the live master before changing the StatefulSet image.
func upgradeCheckScript() string {
	return strings.TrimSpace(`
set -eu
host="${MYSQL_UPGRADE_CHECK_HOST:?}"
port="${MYSQL_UPGRADE_CHECK_PORT:-3306}"
user="${OPERATOR_USER:?}"
pass="${OPERATOR_PASSWORD:?}"
target="${MYSQL_UPGRADE_CHECK_TARGET_VERSION:?}"
echo "MySQL upgrade check against ${host}:${port} (target ${target})"
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
