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
package mysqlcluster

import (
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

// MySQLVersionEnv is set on mysql pods (StatefulSet template) to record the server line they run.
const MySQLVersionEnv = mysqlversioning.MySQLVersionEnv

// Version resolution (use the accessor that matches the question being asked):
//
//   - DesiredVersion: user intent (spec.mysqlVersion → operator default). Use for validation,
//     upgrade targets, and labels that reflect where the cluster is headed.
//
//   - EffectiveVersion: best estimate of what mysqld is running now (status.appliedMysqlVersion →
//     lagging StatefulSet template → DesiredVersion). Use for SQL dialect, GTID helpers, and any
//     logic executed against a live server. status.appliedMysqlVersion lags during rollout until
//     pods are fully updated; the StatefulSet template step covers clusters not yet recorded.
//
//   - RolloutVersion (versionupgrade.RolloutMySQLVersion): version the StatefulSet template should
//     run during upgrade transitions. Holds at the current line when the upgrade path is invalid,
//     otherwise advances to DesiredVersion together with the image roll. Cluster-scoped my.cnf follows
//     RolloutVersion so pods starting on the rolled-forward image get a compatible config.

// DesiredVersion returns the MySQL version the user requested (spec → alias → operator default).
func (c *MysqlCluster) DesiredVersion() semver.Version {
	return mysqlversioning.DesiredVersion(c.Spec.MysqlVersion)
}

// AppliedDataPlaneVersion is status.appliedMysqlVersion after a completed rollout.
func AppliedDataPlaneVersion(c *MysqlCluster) semver.Version {
	return mysqlversioning.AppliedDataPlaneVersion(c.Status.AppliedMysqlVersion)
}

// LaggingStatefulSetVersion returns the MySQL version on the StatefulSet template when it still lags DesiredVersion.
func LaggingStatefulSetVersion(c *MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if sts == nil {
		return semver.Zero
	}
	desired := c.DesiredVersion()
	if v := SemVerFromStatefulSet(sts); !v.IsZero() && !v.EQ(desired) {
		return v
	}
	return semver.Zero
}

// SourceVersionForUpgrade returns status.appliedMysqlVersion (SQL-confirmed data plane only).
func SourceVersionForUpgrade(c *MysqlCluster) semver.Version {
	return AppliedDataPlaneVersion(c)
}

// EffectiveVersion returns the MySQL version running on pods (applied → lagging STS → DesiredVersion).
func (c *MysqlCluster) EffectiveVersion(sts *apps.StatefulSet) semver.Version {
	if v := AppliedDataPlaneVersion(c); !v.IsZero() {
		return v
	}
	if v := LaggingStatefulSetVersion(c, sts); !v.IsZero() {
		return v
	}
	return c.DesiredVersion()
}

// SemVerFromStatefulSet reads MY_MYSQL_VERSION from the StatefulSet pod template, then the mysql
// container image tag (legacy clusters may lack the env var).
func SemVerFromStatefulSet(sts *apps.StatefulSet) semver.Version {
	return mysqlversioning.SemVerFromStatefulSet(sts)
}

// SemVerFromPod reads MY_MYSQL_VERSION from a running mysql pod, then the container image tag.
func SemVerFromPod(pod *core.Pod) semver.Version {
	return mysqlversioning.SemVerFromPod(pod)
}

// ParseServerVersion parses a MySQL server version string (e.g. from SELECT VERSION() or image tags).
func ParseServerVersion(version string) (semver.Version, error) {
	return mysqlversioning.ParseServerVersion(version)
}

// WantsPerconaInitContainerFor reports whether the given server version needs mysql-init-only.
func (c *MysqlCluster) WantsPerconaInitContainerFor(v semver.Version) bool {
	return c.IsPerconaImage() && mysqlversioning.ProfileFor(v).WantsPerconaInitContainer(v)
}
