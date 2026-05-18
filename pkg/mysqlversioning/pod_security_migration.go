/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
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
