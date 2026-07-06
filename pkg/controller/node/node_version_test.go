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
package node

import (
	"strings"
	"testing"

	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestReplicationSQLVersion_usesAppliedDuringUpgrade(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.8"}},
					}},
				},
			},
		},
	}
	pod := &core.Pod{}

	got := replicationSQLVersion(cluster, pod, sts)
	if got.String() != "8.0.20" {
		t.Fatalf("effective version during upgrade: %s", got)
	}
}

func TestReplicationSQLVersion_prefersPodEnvDuringRollout(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	pod := &core.Pod{
		Spec: core.PodSpec{
			Containers: []core.Container{{
				Name: "mysql",
				Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.8"}},
			}},
		},
	}

	got := replicationSQLVersion(cluster, pod, nil)
	if got.String() != "8.4.8" {
		t.Fatalf("pod env should win during rollout: %s", got)
	}
}

func TestReplicationSQLVersion_fallsBackToDesiredOnFreshInstall(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	pod := &core.Pod{}

	got := replicationSQLVersion(cluster, pod, nil)
	if got.String() != "8.4.8" {
		t.Fatalf("fresh install version: %s", got)
	}
}

func TestNewNodeConn_usesDataPlaneDialectDuringUpgrade(t *testing.T) {
	t.Parallel()
	runner := newNodeConn("dsn", "host", mustParseVer(t, "8.0.20"))
	if !strings.Contains(runner.(*nodeSQLRunner).rep.StopReplication, "STOP SLAVE") {
		t.Fatalf("expected master/slave dialect for 8.0, got %q", runner.(*nodeSQLRunner).rep.StopReplication)
	}
}

func mustParseVer(t *testing.T, s string) semver.Version {
	t.Helper()
	v, err := semver.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
