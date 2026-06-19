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

	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"

	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

func TestBuiltinProfileSidecarKeys(t *testing.T) {
	cases := []struct {
		ver  string
		want SidecarProfileKey
	}{
		{"5.7.35", SidecarPercona57},
		{"8.0.20", SidecarPercona80},
		{"8.4.0", SidecarPercona84},
	}
	reg := NewRegistry(BuiltinProfiles())
	for _, tc := range cases {
		v := semver.MustParse(tc.ver)
		if got := reg.MustResolve(v).SidecarProfileKey(); got != tc.want.String() {
			t.Fatalf("version %s: want sidecar key %q, got %q", tc.ver, tc.want, got)
		}
	}
}

func TestInitDefault(t *testing.T) {
	opt := options.GetOptions()
	if err := InitDefault(opt); err != nil {
		t.Fatal(err)
	}
	if ProfileFor(semver.MustParse("8.4.0")).Name() != ProfilePercona84.String() {
		t.Fatalf("expected %s after InitDefault", ProfilePercona84)
	}
}
