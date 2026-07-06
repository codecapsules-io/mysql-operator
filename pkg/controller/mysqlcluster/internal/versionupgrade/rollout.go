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
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
)

// RolloutMySQLVersion is the MySQL version the StatefulSet must run during an upgrade.
// When the upgrade path is invalid the StatefulSet is held at the current running version indefinitely.
func RolloutMySQLVersion(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, pods []core.Pod) semver.Version {
	_ = pods
	desired := cluster.DesiredVersion()
	applied := mysqlcluster.AppliedDataPlaneVersion(cluster)

	if !applied.IsZero() && applied.GT(desired) {
		return applied
	}

	if !mysqlversioning.VersionChangePending(desired, applied, ClusterHasMySQLData(cluster)) {
		return desired
	}

	if applied.IsZero() && ClusterHasMySQLData(cluster) {
		if lag := mysqlcluster.LaggingStatefulSetVersion(cluster, sts); !lag.IsZero() {
			return lag
		}
		if sts != nil {
			if v := mysqlcluster.SemVerFromStatefulSet(sts); !v.IsZero() && !v.EQ(desired) {
				return v
			}
		}
	}

	source := mysqlcluster.SourceVersionForUpgrade(cluster)
	if !source.IsZero() {
		if err := mysqlversioning.ValidateUpgradePath(source, desired); err != nil {
			return source
		}
	}
	return desired
}
