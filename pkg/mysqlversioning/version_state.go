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

import "github.com/codecapsules-io/mysql-operator/pkg/util/semver"

// ProfilesMatch reports whether two versions belong to the same MySQL profile line.
func ProfilesMatch(a, b semver.Version) bool {
	return ProfileFor(a).Name() == ProfileFor(b).Name()
}

// VersionChangePending reports whether desired differs from the SQL-confirmed data plane.
func VersionChangePending(desired, applied semver.Version, hasData bool) bool {
	if applied.IsZero() {
		if !hasData {
			return false
		}
		return true
	}
	if !ProfilesMatch(applied, desired) {
		return true
	}
	return applied.LT(desired)
}
