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

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

func TestResolveMasterOrdinal_singleReplicaWithoutMasterCondition(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
		Spec:       api.MysqlClusterSpec{Replicas: &replicas, SecretName: "sec"},
	})
	ord, err := ResolveMasterOrdinal(cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ord != 0 {
		t.Fatalf("ordinal: %d", ord)
	}
}

func TestResolveMasterOrdinal_multiReplicaWithoutMasterConditionHolds(t *testing.T) {
	replicas := int32(3)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
		Spec:       api.MysqlClusterSpec{Replicas: &replicas, SecretName: "sec"},
	})
	_, err := ResolveMasterOrdinal(cluster)
	if !IsHoldRollout(err) {
		t.Fatalf("expected hold rollout, got: %v", err)
	}
}

func TestResolveMasterOrdinal_multiReplicaWithMasterCondition(t *testing.T) {
	replicas := int32(3)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
		Spec:       api.MysqlClusterSpec{Replicas: &replicas, SecretName: "sec"},
		Status: api.MysqlClusterStatus{
			Nodes: []api.NodeStatus{{
				Name: "demo-mysql-2.mysql.ns",
				Conditions: []api.NodeCondition{{
					Type:   api.NodeConditionMaster,
					Status: core.ConditionTrue,
				}},
			}},
		},
	})
	ord, err := ResolveMasterOrdinal(cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ord != 2 {
		t.Fatalf("ordinal: %d", ord)
	}
}

func TestResolveMasterOrdinal_multiReplicaUnparseableMasterHostHolds(t *testing.T) {
	replicas := int32(2)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"},
		Spec:       api.MysqlClusterSpec{Replicas: &replicas, SecretName: "sec"},
		Status: api.MysqlClusterStatus{
			Nodes: []api.NodeStatus{{
				Name: "not-a-pod-hostname",
				Conditions: []api.NodeCondition{{
					Type:   api.NodeConditionMaster,
					Status: core.ConditionTrue,
				}},
			}},
		},
	})
	_, err := ResolveMasterOrdinal(cluster)
	if !IsHoldRollout(err) {
		t.Fatalf("expected hold rollout, got: %v", err)
	}
}

func TestEnsureJobSteps_holdsWhenOfflineMultiReplicaMasterUnknown(t *testing.T) {
	replicas := int32(2)
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
			Namespace: cluster.Namespace,
		},
		Status: apps.StatefulSetStatus{Replicas: 2},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.0.20"}},
					}},
				},
			},
		},
	}
	c := testClientBuilder().WithObjects(sts).Build()
	err := EnsureJobSteps(context.Background(), c, cluster, sts, options.GetOptions(), PhasePreRollout)
	if !IsHoldRollout(err) {
		t.Fatalf("expected hold rollout, got: %v", err)
	}
}

func TestSourceVersionForUpgrade_usesAppliedNotStatefulSetTemplate(t *testing.T) {
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
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	got := SourceVersionForUpgrade(cluster, sts)
	if got.String() != "8.0.20" {
		t.Fatalf("upgrade source version: %s", got)
	}
}

func TestVersionChangePending_appliedBehindSpecDespiteSTS(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	if !VersionChangePending(cluster, sts) {
		t.Fatal("expected upgrade pending when applied lags spec")
	}
}
