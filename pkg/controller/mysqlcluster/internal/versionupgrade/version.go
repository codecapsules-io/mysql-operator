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
	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
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

// SourceVersionForUpgrade returns the MySQL version to treat as "current" for upgrade validation.
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
}
