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
	apps "k8s.io/api/apps/v1"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

const (
	// upgradeCheckTargetLabel on the check Job records which spec version the Job was created for.
	upgradeCheckTargetLabel = "mysql.presslabs.org/upgrade-check-target-version"

	// authMigrateTargetLabel on the auth migrate Job records which spec version the Job targets.
	authMigrateTargetLabel = "mysql.presslabs.org/auth-migrate-target-version"

	// JobTypeUpgradeCheck is the mysql.presslabs.org/job-type label value for upgrade check Jobs.
	JobTypeUpgradeCheck = "mysql-upgrade-check"
	// JobTypeAuthMigrate is the mysql.presslabs.org/job-type label value for auth plugin migrate Jobs.
	JobTypeAuthMigrate = "mysql-auth-migrate"

	// preRolloutJobsDoneAnnotation records the spec target version for which pre-rollout Jobs succeeded.
	preRolloutJobsDoneAnnotation = "mysql.presslabs.org/pre-rollout-jobs-done-version"
	// postRolloutJobsDoneAnnotation records the spec target version for which post-rollout Jobs succeeded.
	postRolloutJobsDoneAnnotation = "mysql.presslabs.org/post-rollout-jobs-done-version"

	DataVolumeMountPath = constants.DataVolumeMountPath
)

const mySQLVersionEnv = "MY_MYSQL_VERSION"

// AppliedDataPlaneVersion is the operator-recorded MySQL version on the data plane (status.appliedMysqlVersion).
func AppliedDataPlaneVersion(cluster *mysqlcluster.MysqlCluster) semver.Version {
	if cluster.Status.AppliedMysqlVersion == "" {
		return semver.Version{}
	}
	v, err := semver.Parse(cluster.Status.AppliedMysqlVersion)
	if err != nil {
		return semver.Version{}
	}
	return v
}

// AppliedSemVer returns the MySQL version currently applied on the cluster, or semver zero when unknown.
func AppliedSemVer(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if v := AppliedDataPlaneVersion(cluster); !v.EQ(semver.Version{}) {
		return v
	}
	if sts != nil {
		if v := semVerFromStatefulSet(sts); !v.EQ(semver.Version{}) {
			return v
		}
	}
	return semver.Version{}
}

// SourceVersionForUpgrade returns the MySQL version to treat as "current" for upgrade checks.
// Prefer status.appliedMysqlVersion; fall back to a lagging STS template for clusters not yet recorded.
func SourceVersionForUpgrade(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if v := AppliedDataPlaneVersion(cluster); !v.EQ(semver.Version{}) {
		return v
	}
	return laggingStatefulSetVersion(cluster, sts)
}

func semVerFromStatefulSet(sts *apps.StatefulSet) semver.Version {
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name != "mysql" {
			continue
		}
		for _, e := range c.Env {
			if e.Name == mySQLVersionEnv && e.Value != "" {
				if v, err := semver.Parse(e.Value); err == nil {
					return v
				}
			}
		}
	}
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name != "mysql-init-only" {
			continue
		}
		for _, e := range c.Env {
			if e.Name == mySQLVersionEnv && e.Value != "" {
				if v, err := semver.Parse(e.Value); err == nil {
					return v
				}
			}
		}
	}
	return semver.Version{}
}

// DesiredSemVer is the MySQL version from the cluster spec.
func DesiredSemVer(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return cluster.GetMySQLSemVer()
}

// VersionChangePending reports whether spec.mysqlVersion differs from status.appliedMysqlVersion.
func VersionChangePending(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	desired := DesiredSemVer(cluster)
	applied := AppliedDataPlaneVersion(cluster)
	if !applied.EQ(semver.Version{}) {
		return !applied.EQ(desired)
	}
	lag := laggingStatefulSetVersion(cluster, sts)
	if lag.EQ(semver.Version{}) {
		return false
	}
	return !lag.EQ(desired)
}

// MasterDataPVCName returns the PVC name for the current master's data volume.
func MasterDataPVCName(cluster *mysqlcluster.MysqlCluster) string {
	stsName := cluster.GetNameForResource(mysqlcluster.StatefulSet)
	ordinal := masterOrdinal(cluster.GetMasterHost(), stsName)
	return fmt.Sprintf("data-%s-%d", stsName, ordinal)
}

func masterOrdinal(masterHost, stsName string) int32 {
	podName := strings.Split(masterHost, ".")[0]
	prefix := stsName + "-"
	if !strings.HasPrefix(podName, prefix) {
		return 0
	}
	raw := strings.TrimPrefix(podName, prefix)
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

// HasPersistentDataVolume reports whether the cluster stores MySQL data on PVCs.
func HasPersistentDataVolume(cluster *mysqlcluster.MysqlCluster) bool {
	return cluster.Spec.VolumeSpec.PersistentVolumeClaim != nil
}

// ClusterHasMySQLData returns true when the cluster appears to be serving or have served MySQL data.
func ClusterHasMySQLData(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !HasPersistentDataVolume(cluster) {
		return false
	}
	if cluster.Status.ReadyNodes > 0 {
		return true
	}
	if sts != nil && sts.Status.Replicas > 0 {
		return true
	}
	return false
}

// ClusterHasRunningMySQL is true when a mysqld pod is up (online upgrade checks must not mount the data PVC).
func ClusterHasRunningMySQL(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if cluster.Status.ReadyNodes > 0 {
		return true
	}
	if sts != nil && sts.Status.ReadyReplicas > 0 {
		return true
	}
	return false
}

// SetAnnotation sets a single annotation on the cluster object (in-memory).
func SetAnnotation(cluster *mysqlcluster.MysqlCluster, key, value string) {
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[key] = value
}

// MarkAppliedVersion records the version now running on the data plane in status.
func MarkAppliedVersion(cluster *mysqlcluster.MysqlCluster, version semver.Version) {
	cluster.Status.AppliedMysqlVersion = version.String()
	ClearPhaseJobsDoneAnnotations(cluster)
}

func phaseJobsDoneAnnotation(phase Phase) string {
	switch phase {
	case PhasePreRollout:
		return preRolloutJobsDoneAnnotation
	case PhasePostRollout:
		return postRolloutJobsDoneAnnotation
	default:
		return ""
	}
}

// PhaseJobsDoneForTarget is true when Jobs for the phase already succeeded for the given target version
// (recorded before Job objects are deleted).
func PhaseJobsDoneForTarget(cluster *mysqlcluster.MysqlCluster, phase Phase, target semver.Version) bool {
	key := phaseJobsDoneAnnotation(phase)
	if key == "" || target.EQ(semver.Version{}) {
		return false
	}
	return cluster.Annotations[key] == target.String()
}

// MarkPhaseJobsDone records that all required Jobs in the phase succeeded for target.
func MarkPhaseJobsDone(cluster *mysqlcluster.MysqlCluster, phase Phase, target semver.Version) {
	key := phaseJobsDoneAnnotation(phase)
	if key == "" {
		return
	}
	SetAnnotation(cluster, key, target.String())
}

// ClearPhaseJobsDoneAnnotations removes phase completion markers after the upgrade finishes.
func ClearPhaseJobsDoneAnnotations(cluster *mysqlcluster.MysqlCluster) {
	if cluster.Annotations == nil {
		return
	}
	delete(cluster.Annotations, preRolloutJobsDoneAnnotation)
	delete(cluster.Annotations, postRolloutJobsDoneAnnotation)
}

// JobName returns the stable Job name for the cluster's upgrade check.
func JobName(cluster *mysqlcluster.MysqlCluster) string {
	return cluster.GetNameForResource(mysqlcluster.StatefulSet) + "-upgrade-check"
}

// AuthMigrateJobName returns the stable Job name for auth plugin migration.
func AuthMigrateJobName(cluster *mysqlcluster.MysqlCluster) string {
	return cluster.GetNameForResource(mysqlcluster.StatefulSet) + "-auth-migrate"
}

// JobContainerName is the upgrade check Job container name.
const JobContainerName = "upgrade-check"

// AuthMigrateJobContainerName is the auth migrate Job container name.
const AuthMigrateJobContainerName = "auth-migrate"
