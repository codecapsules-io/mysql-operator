/*
Copyright 2026 Pressinfra SRL

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

package mysqlversion

import "github.com/blang/semver"

// AtLeastMySQL8 reports whether the server is MySQL / Percona Server 8.0+ or 9.x.
func AtLeastMySQL8(v semver.Version) bool {
	return v.Major >= 8
}

// AtLeastMySQL8030 gates options removed after MySQL 8.0.29 (e.g. innodb_log_files_in_group,
// --skip-host-cache / host cache server options).
func AtLeastMySQL8030(v semver.Version) bool {
	if v.Major > 8 {
		return true
	}
	if v.Major == 8 && v.Minor > 0 {
		return true
	}
	if v.Major == 8 && v.Minor == 0 && v.Patch >= 30 {
		return true
	}
	return false
}

// AtLeastMySQL84 gates replication terminology and RESET MASTER removal.
func AtLeastMySQL84(v semver.Version) bool {
	if v.Major > 8 {
		return true
	}
	if v.Major == 8 && v.Minor >= 4 {
		return true
	}
	return false
}
