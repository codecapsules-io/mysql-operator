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
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

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
