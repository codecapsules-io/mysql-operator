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
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
)

func TestProfilePodSecurityHints_builtin(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(BuiltinProfiles())
	cases := []struct {
		ver               string
		percona           bool
		wantFS            int64
		wantRunAsUnset    bool
		wantMysqlUID1001 bool
	}{
		{"8.4.8", true, 1001, true, true},
		{"8.4.0", false, 999, false, false},
		{"8.0.34", true, 999, false, false},
		{"5.7.35", true, 999, false, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.ver+"_percona="+boolStr(tc.percona), func(t *testing.T) {
			t.Parallel()
			h := reg.MustResolve(semver.MustParse(tc.ver)).PodSecurityHints(tc.percona)
			if h.FSGroup != tc.wantFS {
				t.Fatalf("FSGroup: want %d got %d", tc.wantFS, h.FSGroup)
			}
			if tc.wantRunAsUnset {
				if h.RunAsUser != nil {
					t.Fatalf("RunAsUser: want nil got %d", *h.RunAsUser)
				}
			} else {
				if h.RunAsUser == nil || *h.RunAsUser != 999 {
					t.Fatalf("RunAsUser: want 999 got %v", h.RunAsUser)
				}
			}
			if tc.wantMysqlUID1001 {
				if h.MysqlRunAsUser == nil || *h.MysqlRunAsUser != 1001 ||
					h.MysqlRunAsGroup == nil || *h.MysqlRunAsGroup != 1001 {
					t.Fatalf("MysqlRunAs: want 1001/1001 got user=%v group=%v", h.MysqlRunAsUser, h.MysqlRunAsGroup)
				}
			} else {
				if h.MysqlRunAsUser != nil || h.MysqlRunAsGroup != nil {
					t.Fatalf("MysqlRunAs: want nil got user=%v group=%v", h.MysqlRunAsUser, h.MysqlRunAsGroup)
				}
			}
		})
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestProfilesWithOverlay_podSecurityDelegatesToBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "overlay.yaml")
	content := `prependProfiles:
  - name: test-10x
    semverRange: ">=10.0.0 <11.0.0"
    baseProfile: ` + ProfilePercona84.String() + `
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	profs, err := ProfilesWithOverlay(p)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(profs)
	v := semver.MustParse("10.1.0")
	h := reg.MustResolve(v).PodSecurityHints(true)
	if h.FSGroup != 1001 || h.RunAsUser != nil || h.MysqlRunAsUser == nil || *h.MysqlRunAsUser != 1001 {
		t.Fatalf("overlay on percona-8.4 base: want fsGroup 1001, nil pod RunAsUser, mysql 1001, got %+v", h)
	}
}
