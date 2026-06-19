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

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
)

// AppliedDataPlaneVersion is the operator-recorded MySQL version on the data plane (status.appliedMysqlVersion).
func AppliedDataPlaneVersion(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return mysqlcluster.AppliedDataPlaneVersion(cluster)
}

// SourceVersionForUpgrade returns status.appliedMysqlVersion (SQL-confirmed data plane only).
func SourceVersionForUpgrade(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return mysqlcluster.SourceVersionForUpgrade(cluster)
}

// DesiredSemVer is the user-requested MySQL version (spec → operator default).
func DesiredSemVer(cluster *mysqlcluster.MysqlCluster) semver.Version {
	return cluster.DesiredVersion()
}

// VersionChangePending reports whether spec.mysqlVersion differs from the SQL-confirmed data plane.
func VersionChangePending(cluster *mysqlcluster.MysqlCluster) bool {
	desired := DesiredSemVer(cluster)
	applied := AppliedDataPlaneVersion(cluster)
	if applied.EQ(semver.Version{}) {
		if !ClusterHasMySQLData(cluster) {
			return false
		}
		return true
	}
	appliedProfile := mysqlversioning.ProfileFor(applied).Name()
	desiredProfile := mysqlversioning.ProfileFor(desired).Name()
	if appliedProfile != desiredProfile {
		return true
	}
	return applied.LT(desired)
}

// HasPersistentDataVolume reports whether the cluster stores MySQL data on PVCs.
func HasPersistentDataVolume(cluster *mysqlcluster.MysqlCluster) bool {
	return cluster.Spec.VolumeSpec.PersistentVolumeClaim != nil
}

// ClusterHasMySQLData returns true when the cluster appears to be serving MySQL data.
func ClusterHasMySQLData(cluster *mysqlcluster.MysqlCluster) bool {
	if !HasPersistentDataVolume(cluster) {
		return false
	}
	return cluster.Status.ReadyNodes > 0
}

// MarkAppliedVersion records the version now running on the data plane in status.
func MarkAppliedVersion(cluster *mysqlcluster.MysqlCluster, version semver.Version) {
	cluster.Status.AppliedMysqlVersion = version.String()
}
