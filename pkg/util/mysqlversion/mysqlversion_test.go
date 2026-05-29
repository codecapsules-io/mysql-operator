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
package mysqlversion

import (
	"testing"

	"github.com/blang/semver"
)

func TestAtLeastMySQL8030(t *testing.T) {
	cases := []struct {
		v   string
		exp bool
	}{
		{"8.0.29", false},
		{"8.0.30", true},
		{"8.1.0", true},
		{"9.0.0", true},
	}
	for _, tc := range cases {
		sv := semver.MustParse(tc.v)
		if got := AtLeastMySQL8030(sv); got != tc.exp {
			t.Fatalf("%s: got %v want %v", tc.v, got, tc.exp)
		}
	}
}

func TestAtLeastMySQL84(t *testing.T) {
	if !AtLeastMySQL84(semver.MustParse("8.4.0")) {
		t.Fatal()
	}
	if AtLeastMySQL84(semver.MustParse("8.3.9")) {
		t.Fatal()
	}
	if !AtLeastMySQL84(semver.MustParse("9.0.0")) {
		t.Fatal()
	}
}
