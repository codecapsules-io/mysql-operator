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
)

func TestInnoDBOperatorLogSizing_builtinProfiles(t *testing.T) {
	const perFile = 128 << 20 // 128Mi
	reg := NewRegistry(BuiltinProfiles())
	cases := []struct {
		ver      string
		wantKey  string
		wantSize int64
	}{
		{"5.7.35", "innodb-log-file-size", perFile},
		{"8.0.20", "innodb-log-file-size", perFile},
		{"8.0.30", "innodb-redo-log-capacity", 2 * perFile},
		{"8.4.0", "innodb-redo-log-capacity", 2 * perFile},
	}
	for _, tc := range cases {
		v := semver.MustParse(tc.ver)
		p := reg.MustResolve(v)
		key, sz := p.InnoDBOperatorLogSizing(v, perFile)
		if key != tc.wantKey || sz != tc.wantSize {
			t.Fatalf("%s profile %q: got (%q, %d), want (%q, %d)", tc.ver, p.Name(), key, sz, tc.wantKey, tc.wantSize)
		}
	}
}

func TestInnoDBOperatorLogSizing_fallbackByVersion(t *testing.T) {
	const perFile = 64 << 20
	reg := NewRegistry([]Profile{profileFallback{}})
	vPre := semver.MustParse("8.0.29")
	p := reg.MustResolve(vPre)
	if k, sz := p.InnoDBOperatorLogSizing(vPre, perFile); k != "innodb-log-file-size" || sz != perFile {
		t.Fatalf("8.0.29 fallback: got %q %d", k, sz)
	}
	vPost := semver.MustParse("8.0.30")
	if k, sz := p.InnoDBOperatorLogSizing(vPost, perFile); k != "innodb-redo-log-capacity" || sz != 2*perFile {
		t.Fatalf("8.0.30 fallback: got %q %d", k, sz)
	}
}
