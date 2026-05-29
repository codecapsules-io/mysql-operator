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
package versionupgrade

import (
	"context"
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func TestRolloutMySQLVersion_holdsUntilCheckPasses(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	c := testClientBuilder().Build()
	got := RolloutMySQLVersion(context.Background(), c, cluster, sts)
	if got.String() != "8.0.20" {
		t.Fatalf("rollout version: %s", got)
	}
	c = testClientBuilder().WithObjects(
		upgradeCheckJobSucceeded(cluster, "8.4.0"),
		authMigrateJobSucceeded(cluster, "8.4.0"),
	).Build()
	got = RolloutMySQLVersion(context.Background(), c, cluster, sts)
	if got.String() != "8.4.0" {
		t.Fatalf("after check: %s", got)
	}
}

func TestNeedsDatadirChownInit(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			Image:        "percona/percona-server:8.4",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	sts := &apps.StatefulSet{Status: apps.StatefulSetStatus{Replicas: 1}}
	c := testClientBuilder().WithObjects(
		upgradeCheckJobSucceeded(cluster, "8.4.0"),
		authMigrateJobSucceeded(cluster, "8.4.0"),
	).Build()
	if !NeedsDatadirChownInit(context.Background(), c, cluster, sts) {
		t.Fatal("expected chown init when upgrading 8.0 Percona to 8.4")
	}
}

func TestNeedsDatadirChownInit_requiresPersistentData(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			Image:        "percona/percona-server:8.4",
		},
	})
	c := testClientBuilder().WithObjects(upgradeCheckJobSucceeded(cluster, "8.4.0")).Build()
	if NeedsDatadirChownInit(context.Background(), c, cluster, nil) {
		t.Fatal("expected no chown without a persistent volume")
	}
}
