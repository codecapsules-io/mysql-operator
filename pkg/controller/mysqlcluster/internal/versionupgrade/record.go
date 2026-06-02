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

func rolloutCompleteOnVersion(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, version semver.Version) bool {
	if sts == nil || cluster.Spec.Replicas == nil {
		return false
	}
	replicas := *cluster.Spec.Replicas
	if replicas == 0 {
		return false
	}
	if !semVerFromStatefulSet(sts).EQ(version) {
		return false
	}
	return int(sts.Status.ReadyReplicas) >= int(replicas)
}

// laggingStatefulSetVersion returns the MySQL version on the STS template when it still lags spec.mysqlVersion.
func laggingStatefulSetVersion(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if sts == nil {
		return semver.Version{}
	}
	desired := DesiredSemVer(cluster)
	if v := semVerFromStatefulSet(sts); !v.EQ(semver.Version{}) && !v.EQ(desired) {
		return v
	}
	return semver.Version{}
}
