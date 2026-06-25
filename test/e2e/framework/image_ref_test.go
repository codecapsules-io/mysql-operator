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

package framework

import (
	"reflect"
	"testing"
)

func TestNormalizeImageRef(t *testing.T) {
	cases := []struct {
		ref  string
		want []string
	}{
		{
			ref:  "mysql-operator:local",
			want: []string{"mysql-operator:local", "docker.io/library/mysql-operator:local"},
		},
		{
			ref:  "prom/mysqld-exporter:v0.16.0",
			want: []string{"prom/mysqld-exporter:v0.16.0", "docker.io/prom/mysqld-exporter:v0.16.0"},
		},
		{
			ref:  "kind-e2e/percona-5.7:local",
			want: []string{"kind-e2e/percona-5.7:local", "docker.io/kind-e2e/percona-5.7:local"},
		},
		{
			ref:  "docker.io/library/mysql-operator:local",
			want: []string{"docker.io/library/mysql-operator:local"},
		},
	}
	for _, tc := range cases {
		got := NormalizeImageRef(tc.ref)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("NormalizeImageRef(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

func TestImageRefPresentInCrictl(t *testing.T) {
	crictlOut := `"repoTags":["docker.io/library/mysql-operator:local"]`
	if !imageRefPresentInCrictl(crictlOut, "mysql-operator:local") {
		t.Fatal("expected mysql-operator:local to match canonical crictl ref")
	}
	if imageRefPresentInCrictl(crictlOut, "mysql-operator:missing") {
		t.Fatal("unexpected match for missing image")
	}
}
