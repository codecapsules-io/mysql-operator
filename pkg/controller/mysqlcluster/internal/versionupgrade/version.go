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
	core "k8s.io/api/core/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/apis/domain"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

const (
	upgradeCheckTargetLabel = domain.LabelUpgradeCheckTargetVersion

	JobTypeUpgradeCheck = domain.JobTypeUpgradeCheck

	preRolloutJobsDoneAnnotation  = domain.AnnotationPreRolloutJobsDone
	postRolloutJobsDoneAnnotation = domain.AnnotationPostRolloutJobsDone

	DataVolumeMountPath = constants.DataVolumeMountPath
)

// AppliedDataPlaneVersion is the operator-recorded MySQL version on the data plane (status.appliedMysqlVersion).
func AppliedDataPlaneVersion(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return mysqlcluster.AppliedDataPlaneVersion(cluster)
}

// AppliedSemVer returns the MySQL version currently applied on the cluster, or semver zero when unknown.
func AppliedSemVer(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if v := AppliedDataPlaneVersion(cluster); !v.EQ(semver.Version{}) {
		return v
	}
	if sts != nil {
		if v := mysqlcluster.SemVerFromStatefulSet(sts); !v.EQ(semver.Version{}) {
			return v
		}
	}
	return semver.Version{}
}

// SourceVersionForUpgrade returns the MySQL version to treat as "current" for upgrade checks.
// Prefer status.appliedMysqlVersion; fall back to a lagging STS template for clusters not yet recorded.
func SourceVersionForUpgrade(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	return mysqlcluster.SourceVersionForUpgrade(cluster, sts)
}

// DesiredSemVer is the user-requested MySQL version (spec → operator default).
func DesiredSemVer(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return cluster.DesiredVersion()
}

// VersionChangePending reports whether spec.mysqlVersion differs from status.appliedMysqlVersion.
func VersionChangePending(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	desired := DesiredSemVer(cluster)
	applied := AppliedDataPlaneVersion(cluster)
	if !applied.EQ(semver.Version{}) {
		return !applied.EQ(desired)
	}
	lag := mysqlcluster.LaggingStatefulSetVersion(cluster, sts)
	if lag.EQ(semver.Version{}) {
		return false
	}
	return !lag.EQ(desired)
}

// MasterDataPVCName returns the PVC name for the current master's data volume.
func MasterDataPVCName(cluster *mysqlcluster.MysqlCluster) (string, error) {
	ord, err := ResolveMasterOrdinal(cluster)
	if err != nil {
		return "", err
	}
	stsName := cluster.GetNameForResource(mysqlcluster.StatefulSet)
	return fmt.Sprintf("data-%s-%d", stsName, ord), nil
}

// ResolveMasterOrdinal returns the StatefulSet pod ordinal for the writable primary.
// On multi-replica clusters, returns HoldRolloutError when master identity is unknown.
func ResolveMasterOrdinal(cluster *mysqlcluster.MysqlCluster) (int32, error) {
	if !isMultiReplica(cluster) {
		return 0, nil
	}
	masterHost, ok := masterHostFromStatus(cluster)
	if !ok {
		return 0, &HoldRolloutError{
			Reason: "waiting for MySQL master to be identified before offline upgrade check (no Master condition in status.nodes)",
		}
	}
	stsName := cluster.GetNameForResource(mysqlcluster.StatefulSet)
	ord, ok := parseMasterOrdinal(masterHost, stsName)
	if !ok {
		return 0, &HoldRolloutError{
			Reason: fmt.Sprintf("waiting for MySQL master identity: cannot resolve master ordinal from host %q", masterHost),
		}
	}
	return ord, nil
}

func isMultiReplica(cluster *mysqlcluster.MysqlCluster) bool {
	replicas := cluster.Spec.Replicas
	return replicas != nil && *replicas > 1
}

func masterHostFromStatus(cluster *mysqlcluster.MysqlCluster) (string, bool) {
	for _, ns := range cluster.Status.Nodes {
		if cond := cluster.GetNodeCondition(ns.Name, api.NodeConditionMaster); cond != nil &&
			cond.Status == core.ConditionTrue {
			return ns.Name, true
		}
	}
	return "", false
}

func parseMasterOrdinal(masterHost, stsName string) (int32, bool) {
	podName := strings.Split(masterHost, ".")[0]
	prefix := stsName + "-"
	if !strings.HasPrefix(podName, prefix) {
		return 0, false
	}
	raw := strings.TrimPrefix(podName, prefix)
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(n), true
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

// JobContainerName is the upgrade check Job container name.
const JobContainerName = "upgrade-check"
