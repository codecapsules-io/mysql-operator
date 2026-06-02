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
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestBuildBashPreStop_usesAppliedVersionDuringUpgrade(t *testing.T) {
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.4.0",
		},
		Status: api.MysqlClusterStatus{
			AppliedMysqlVersion: "8.0.20",
		},
	})

	script := buildBashPreStop(cluster, nil)
	if !strings.Contains(script, "SHOW SLAVE STATUS") {
		t.Fatalf("expected master/slave preStop SQL while applied is 8.0, got: %s", script)
	}
	if strings.Contains(script, "SHOW REPLICA STATUS") {
		t.Fatalf("did not expect replica terminology preStop while applied is 8.0")
	}
}

func TestBuildBashPreStop_usesSpecWhenAppliedMatches(t *testing.T) {
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.4.0",
		},
		Status: api.MysqlClusterStatus{
			AppliedMysqlVersion: "8.4.0",
		},
	})

	script := buildBashPreStop(cluster, nil)
	if !strings.Contains(script, "SHOW REPLICA STATUS") {
		t.Fatalf("expected replica preStop SQL when applied is 8.4, got: %s", script)
	}
}
