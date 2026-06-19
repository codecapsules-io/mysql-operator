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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func testClientBuilder() *fake.ClientBuilder {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = api.SchemeBuilder.AddToScheme(s)
	_ = apps.AddToScheme(s)
	return fake.NewClientBuilder().WithScheme(s)
}

func TestEnsureChecked_noChange(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0.20",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	if err := EnsureChecked(context.Background(), c, cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureChecked_freshClusterNoAppliedDoesNotDeadlock(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4",
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
		Status: apps.StatefulSetStatus{Replicas: 1},
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
	c := testClientBuilder().WithObjects(sts).Build()
	if err := EnsureChecked(context.Background(), c, cluster); err != nil {
		t.Fatalf("fresh install must not block on unknown source: %v", err)
	}
}

func TestEnsureChecked_patchBumpSucceeds(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20", ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0.34",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	if err := EnsureChecked(context.Background(), c, cluster); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureChecked_legacyClusterWithoutAppliedHoldsUntilBackfill(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
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
		Status: apps.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name:  "mysql",
						Image: "docker.io/percona/percona-server:8.0.34",
					}},
				},
			},
		},
	}
	c := testClientBuilder().WithObjects(sts).Build()
	err := EnsureChecked(context.Background(), c, cluster)
	if !IsHoldRollout(err) {
		t.Fatalf("legacy cluster without applied must hold until SQL backfill: %v", err)
	}
}

func TestEnsureChecked_blocksSkipLine(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "5.7.35", ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	err := EnsureChecked(context.Background(), c, cluster)
	if err == nil {
		t.Fatal("expected error for skipped LTS line")
	}
}

func TestSyncAppliedVersion(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0.34",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					InitContainers: []core.Container{{Name: "init"}},
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.0.34"}},
					}},
				},
			},
		},
	}
	pod := mysqlReadyPod("c1-mysql-0")
	pod.Spec.InitContainers = []core.Container{{Name: "init"}}
	pod.Status.InitContainerStatuses = []core.ContainerStatus{{
		Name:  "init",
		State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 0}},
	}}
	secret := testOperatorSecret(cluster)
	withMockMysqldVersion("8.0.34-26", func() {
		c := testClientBuilder().WithObjects(secret).Build()
		advance, ok := SyncAppliedVersion(context.Background(), c, cluster, sts, []core.Pod{pod})
		if !ok {
			t.Fatal("expected rollout to be ready for applied version update")
		}
		if advance.String() != "8.0.34" {
			t.Fatalf("advance version: %s", advance)
		}
	})
}
