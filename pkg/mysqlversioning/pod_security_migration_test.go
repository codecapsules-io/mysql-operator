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
	"testing"

	"github.com/blang/semver"
)

func TestNeedsDatadirOwnershipMigration(t *testing.T) {
	cases := []struct {
		applied  string
		desired  string
		percona  bool
		want     bool
	}{
		{"8.0.20", "8.4.0", true, true},
		{"5.7.35", "8.4.0", true, true},
		{"8.4.0", "8.4.0", true, false},
		{"8.0.20", "8.0.34", true, false},
		{"8.0.20", "8.4.0", false, false},
		{"", "8.4.0", true, false},
	}
	for _, tc := range cases {
		var applied semver.Version
		if tc.applied != "" {
			applied = semver.MustParse(tc.applied)
		}
		got := NeedsDatadirOwnershipMigration(applied, semver.MustParse(tc.desired), tc.percona)
		if got != tc.want {
			t.Fatalf("%s -> %s percona=%v: got %v want %v", tc.applied, tc.desired, tc.percona, got, tc.want)
		}
	}
}
