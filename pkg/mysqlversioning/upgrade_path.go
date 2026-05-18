/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"fmt"

	"github.com/blang/semver"
)

// ValidateUpgradePath returns an error when changing MySQL server version from current to target
// is not supported (downgrade or skipping an LTS line).
func ValidateUpgradePath(current, target semver.Version) error {
	if target.LT(current) {
		return fmt.Errorf("downgrading MySQL from %s to %s is not supported", current, target)
	}
	if target.EQ(current) {
		return nil
	}

	fromName := ProfileFor(current).Name()
	toName := ProfileFor(target).Name()
	fromIdx, fromOK := profileUpgradeIndex(fromName)
	toIdx, toOK := profileUpgradeIndex(toName)
	if !fromOK || !toOK {
		return fmt.Errorf("cannot validate upgrade path from profile %q to %q", fromName, toName)
	}
	if toIdx > fromIdx+1 {
		return fmt.Errorf(
			"upgrade from MySQL %s (%s) to %s (%s) must be done one LTS line at a time (e.g. 8.0.x before 8.4.x)",
			current, fromName, target, toName,
		)
	}
	return nil
}

// NeedsDatadirUpgradeCheck is true when the target version line differs from the current line and an
// offline datadir check with the target server binary should run before changing the StatefulSet image.
func NeedsDatadirUpgradeCheck(current, target semver.Version) bool {
	if target.EQ(current) {
		return false
	}
	return ProfileFor(current).Name() != ProfileFor(target).Name()
}

// NeedsAuthPluginMigration is true when accounts may still use mysql_native_password on the source
// line but the target line no longer loads that plugin (e.g. Percona 8.0 → 8.4).
func NeedsAuthPluginMigration(current, target semver.Version) bool {
	if current.EQ(semver.Version{}) || target.EQ(current) {
		return false
	}
	if !ProfileFor(current).UseMySQL80AuthPlugin() {
		return false
	}
	return !ProfileFor(target).UseMySQL80AuthPlugin()
}
