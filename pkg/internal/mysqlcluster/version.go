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
	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

// MySQLVersionEnv is set on mysql pods (StatefulSet template) to record the server line they run.
const MySQLVersionEnv = "MY_MYSQL_VERSION"

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
//   - RolloutVersion (versionupgrade.RolloutMySQLVersion): version the StatefulSet template and
//     my.cnf should emit during upgrade transitions. Holds at the current line when the upgrade
//     path is invalid, otherwise advances to DesiredVersion together with the image roll.

// DesiredVersion returns the MySQL version the user requested (spec → alias → operator default).
func (c *MysqlCluster) DesiredVersion() semver.Version {
	version := c.Spec.MysqlVersion
	if version == "" {
		version = constants.MySQLDefaultVersion
	}
	if v, ok := constants.MySQLTagsToSemVer[version]; ok {
		version = v
	}
	sv, err := semver.Make(version)
	if err != nil {
		log.Error(err, "failed to parse given MySQL version", "input", version)
	}
	return sv
}

// AppliedDataPlaneVersion is status.appliedMysqlVersion after a completed rollout.
func AppliedDataPlaneVersion(c *MysqlCluster) semver.Version {
	if c.Status.AppliedMysqlVersion == "" {
		return semver.Version{}
	}
	v, err := semver.Parse(c.Status.AppliedMysqlVersion)
	if err != nil {
		return semver.Version{}
	}
	return v
}

// LaggingStatefulSetVersion returns the MySQL version on the StatefulSet template when it still lags DesiredVersion.
func LaggingStatefulSetVersion(c *MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if sts == nil {
		return semver.Version{}
	}
	desired := c.DesiredVersion()
	if v := SemVerFromStatefulSet(sts); !v.EQ(semver.Version{}) && !v.EQ(desired) {
		return v
	}
	return semver.Version{}
}

// SourceVersionForUpgrade returns the known "from" version for upgrade-path checks.
// Unlike EffectiveVersion, it does not fall back to spec when neither applied nor STS is known.
func SourceVersionForUpgrade(c *MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if v := AppliedDataPlaneVersion(c); !v.EQ(semver.Version{}) {
		return v
	}
	return LaggingStatefulSetVersion(c, sts)
}

// EffectiveVersion returns the MySQL version running on pods (applied → lagging STS → DesiredVersion).
func (c *MysqlCluster) EffectiveVersion(sts *apps.StatefulSet) semver.Version {
	if v := SourceVersionForUpgrade(c, sts); !v.EQ(semver.Version{}) {
		return v
	}
	return c.DesiredVersion()
}

// SemVerFromStatefulSet reads MySQLVersionEnv from the StatefulSet pod template.
func SemVerFromStatefulSet(sts *apps.StatefulSet) semver.Version {
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name != "mysql" {
			continue
		}
		for _, e := range c.Env {
			if e.Name == MySQLVersionEnv && e.Value != "" {
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
			if e.Name == MySQLVersionEnv && e.Value != "" {
				if v, err := semver.Parse(e.Value); err == nil {
					return v
				}
			}
		}
	}
	return semver.Version{}
}

// SemVerFromPod reads MySQLVersionEnv from a running mysql pod (preferred for per-pod SQL dialect).
func SemVerFromPod(pod *core.Pod) semver.Version {
	if pod == nil {
		return semver.Version{}
	}
	for _, c := range pod.Spec.Containers {
		if c.Name != "mysql" {
			continue
		}
		for _, e := range c.Env {
			if e.Name == MySQLVersionEnv && e.Value != "" {
				if v, err := semver.Parse(e.Value); err == nil {
					return v
				}
			}
		}
	}
	return semver.Version{}
}

// WantsPerconaInitContainerFor reports whether the given server version needs mysql-init-only.
func (c *MysqlCluster) WantsPerconaInitContainerFor(v semver.Version) bool {
	return c.IsPerconaImage() && mysqlversioning.ProfileFor(v).WantsPerconaInitContainer(v)
}
