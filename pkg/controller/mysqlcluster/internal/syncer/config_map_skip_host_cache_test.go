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
package mysqlcluster

import (
	"strings"
	"testing"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	icluster "github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildMysqlConfData_skipHostCacheByVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version      string
		wantSkipHost bool
	}{
		{"8.0.20", true},
		{"8.0.29", true},
		{"8.0.30", false},
		{"8.4.8", false},
	} {
		tc := tc
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			c := icluster.New(&api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
				Spec: api.MysqlClusterSpec{
					MysqlVersion: tc.version,
					SecretName:   "sec",
				},
			})
			data, err := buildMysqlConfData(c)
			if err != nil {
				t.Fatal(err)
			}
			has := strings.Contains(data, "skip-host-cache")
			if has != tc.wantSkipHost {
				t.Fatalf("skip-host-cache present=%v want present=%v", has, tc.wantSkipHost)
			}
			if !strings.Contains(data, "skip-name-resolve") {
				t.Fatal("expected skip-name-resolve in generated my.cnf")
			}
		})
	}
}
