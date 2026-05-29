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

import "github.com/blang/semver"

// NeedsDatadirOwnershipMigration is true when upgrading onto a profile that runs mysqld as UID/GID
// 1001 (Percona Server 8.4+) from an older line that used 999, so binlogs and data files must be chowned.
func NeedsDatadirOwnershipMigration(applied, desired semver.Version, perconaServerImage bool) bool {
	newHints := ProfileFor(desired).PodSecurityHints(perconaServerImage)
	if newHints.MysqlRunAsUser == nil || *newHints.MysqlRunAsUser != perconaUIDGID1001 {
		return false
	}
	if applied.EQ(semver.Version{}) {
		return false
	}
	oldHints := ProfileFor(applied).PodSecurityHints(perconaServerImage)
	if oldHints.MysqlRunAsUser != nil && *oldHints.MysqlRunAsUser == perconaUIDGID1001 {
		return false
	}
	return true
}
