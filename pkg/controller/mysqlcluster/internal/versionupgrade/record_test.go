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
)

func TestSyncAppliedVersion_waitsUntilRolloutComplete(t *testing.T) {
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
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					InitContainers: []core.Container{{
						Name:    DatadirChownInitContainerName,
						Command: []string{"/bin/sh"},
					}},
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	c := testClientBuilder().
		WithObjects(upgradeCheckJobSucceeded(cluster, "8.4.0")).
		Build()
	if SyncAppliedVersion(context.Background(), c, cluster, sts, nil) {
		t.Fatal("should not set applied until init containers succeed on pods")
	}
	if cluster.Status.AppliedMysqlVersion != "8.0.20" {
		t.Fatalf("applied version: %q", cluster.Status.AppliedMysqlVersion)
	}
}

func TestSyncAppliedVersion_afterFullRollout(t *testing.T) {
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
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					InitContainers: []core.Container{{
						Name:    DatadirChownInitContainerName,
						Command: []string{"/bin/sh"},
					}},
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mysqlcluster.MySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	c := testClientBuilder().
		WithObjects(upgradeCheckJobSucceeded(cluster, "8.4.0")).
		Build()
	pod := core.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-mysql-0"},
		Spec: core.PodSpec{
			InitContainers: []core.Container{
				{Name: DatadirChownInitContainerName},
				{Name: "init"},
			},
		},
		Status: core.PodStatus{
			InitContainerStatuses: []core.ContainerStatus{
				{
					Name: DatadirChownInitContainerName,
					State: core.ContainerState{
						Terminated: &core.ContainerStateTerminated{ExitCode: 0},
					},
				},
				{
					Name: "init",
					State: core.ContainerState{
						Terminated: &core.ContainerStateTerminated{ExitCode: 0},
					},
				},
			},
		},
	}
	if !SyncAppliedVersion(context.Background(), c, cluster, sts, []core.Pod{pod}) {
		t.Fatal("expected applied version after full rollout")
	}
	if cluster.Status.AppliedMysqlVersion != "8.4.0" {
		t.Fatalf("applied version: %q", cluster.Status.AppliedMysqlVersion)
	}
}
