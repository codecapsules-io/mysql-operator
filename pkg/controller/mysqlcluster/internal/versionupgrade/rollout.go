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
	"context"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

// RolloutMySQLVersion is the MySQL version the StatefulSet must run until the upgrade check passes.
// After the check passes it matches spec.mysqlVersion so chown init and the new image roll out together.
func RolloutMySQLVersion(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	desired := DesiredSemVer(cluster)
	if !VersionChangePending(cluster, sts) {
		return desired
	}
	if JobStepsComplete(ctx, c, cluster, sts, PhasePreRollout) {
		return desired
	}
	if v := AppliedDataPlaneVersion(cluster); !v.EQ(semver.Version{}) {
		return v
	}
	if v := laggingStatefulSetVersion(cluster, sts); !v.EQ(semver.Version{}) {
		return v
	}
	return desired
}
