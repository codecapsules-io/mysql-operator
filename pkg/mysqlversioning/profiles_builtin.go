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
package mysqlversioning

import (
	"github.com/blang/semver"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/util/mysqlversion"
)

var perconaInitRange = semver.MustParseRange(">=5.7.26 <8.0.0 || >=8.0.15")

type profilePercona57 struct{}

func (profilePercona57) Name() string { return ProfilePercona57.String() }

func (profilePercona57) Matches(v semver.Version) bool {
	return v.Major == 5
}

func (profilePercona57) MySQLOperatorKV(v semver.Version) map[string]string {
	return OperatorKVCommon(v, false)
}

func (profilePercona57) UseMySQL5xConfigs() bool { return true }
func (profilePercona57) UseMySQL8xConfigs() bool { return false }
func (profilePercona57) UseMySQL80AuthPlugin() bool {
	return false
}

func (profilePercona57) Replication() ReplicationDialect { return MasterSlaveReplication() }
func (profilePercona57) GrantHints() GrantHints          { return masterSlaveGrantHintsMySQL57() }
func (profilePercona57) SidecarProfileKey() string       { return SidecarPercona57.String() }

func (profilePercona57) WantsPerconaInitContainer(v semver.Version) bool {
	return perconaInitRange(v)
}

func (profilePercona57) Validate(spec *api.MysqlClusterSpec) error {
	return DefaultValidate(spec)
}

func (profilePercona57) PodSecurityHints(perconaServerImage bool) PodSecurityHints {
	_ = perconaServerImage
	return PodSecurityLegacy999()
}

func (profilePercona57) InnoDBOperatorLogSizing(_ semver.Version, perFileBytes int64) (string, int64) {
	return "innodb-log-file-size", perFileBytes
}

type profilePercona80 struct{}

func (profilePercona80) Name() string { return ProfilePercona80.String() }

func (profilePercona80) Matches(v semver.Version) bool {
	return mysqlversion.AtLeastMySQL8(v) && !mysqlversion.AtLeastMySQL84(v)
}

func (profilePercona80) MySQLOperatorKV(v semver.Version) map[string]string {
	return OperatorKVCommon(v, false)
}

func (profilePercona80) UseMySQL5xConfigs() bool { return false }
func (profilePercona80) UseMySQL8xConfigs() bool { return true }
func (profilePercona80) UseMySQL80AuthPlugin() bool {
	return true
}

func (profilePercona80) Replication() ReplicationDialect { return MasterSlaveReplication() }
func (profilePercona80) GrantHints() GrantHints          { return masterSlaveGrantHintsMySQL8() }
func (profilePercona80) SidecarProfileKey() string       { return SidecarPercona80.String() }

func (profilePercona80) WantsPerconaInitContainer(v semver.Version) bool {
	return perconaInitRange(v)
}

func (profilePercona80) Validate(spec *api.MysqlClusterSpec) error {
	return DefaultValidate(spec)
}

func (profilePercona80) PodSecurityHints(perconaServerImage bool) PodSecurityHints {
	_ = perconaServerImage
	return PodSecurityLegacy999()
}

func (profilePercona80) InnoDBOperatorLogSizing(v semver.Version, perFileBytes int64) (string, int64) {
	if mysqlversion.AtLeastMySQL8030(v) {
		return "innodb-redo-log-capacity", 2 * perFileBytes
	}
	return "innodb-log-file-size", perFileBytes
}

type profilePercona84 struct{}

func (profilePercona84) Name() string { return ProfilePercona84.String() }

func (profilePercona84) Matches(v semver.Version) bool {
	return mysqlversion.AtLeastMySQL84(v)
}

func (profilePercona84) MySQLOperatorKV(v semver.Version) map[string]string {
	return OperatorKVCommon(v, true)
}

func (profilePercona84) UseMySQL5xConfigs() bool { return false }
func (profilePercona84) UseMySQL8xConfigs() bool { return true }
func (profilePercona84) UseMySQL80AuthPlugin() bool {
	return false
}

func (profilePercona84) Replication() ReplicationDialect { return SourceReplicaReplication() }
func (profilePercona84) GrantHints() GrantHints          { return sourceReplicaGrantHints() }
func (profilePercona84) SidecarProfileKey() string       { return SidecarPercona84.String() }

func (profilePercona84) WantsPerconaInitContainer(v semver.Version) bool {
	return perconaInitRange(v)
}

func (profilePercona84) Validate(spec *api.MysqlClusterSpec) error {
	return DefaultValidate(spec)
}

func (profilePercona84) PodSecurityHints(perconaServerImage bool) PodSecurityHints {
	if perconaServerImage {
		return PodSecurityPerconaUID1001VolumeGroup()
	}
	return PodSecurityLegacy999()
}

func (profilePercona84) InnoDBOperatorLogSizing(_ semver.Version, perFileBytes int64) (string, int64) {
	return "innodb-redo-log-capacity", 2 * perFileBytes
}

// BuiltinProfiles returns built-in Percona profiles in match order (first match wins),
// terminated by a catch-all fallback for unknown semvers.
func BuiltinProfiles() []Profile {
	return []Profile{
		profilePercona84{},
		profilePercona80{},
		profilePercona57{},
		profileFallback{},
	}
}
