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

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestDeleteSucceededJobStepsForPhase_removesSucceededPreRolloutJobs(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
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
	upJob := upgradeCheckJobSucceeded(cluster, "8.4.0")
	c := testClientBuilder().WithObjects(upJob).Build()
	MarkPhaseJobsDone(cluster, PhasePreRollout, semver.MustParse("8.4.0"))

	if err := DeleteSucceededJobStepsForPhase(context.Background(), c, cluster, sts, PhasePreRollout); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, name := range []string{JobName(cluster)} {
		j := &batch.Job{}
		key := types.NamespacedName{Name: name, Namespace: cluster.Namespace}
		if err := c.Get(context.Background(), key, j); !errors.IsNotFound(err) {
			t.Fatalf("job %s: expected NotFound, got %v", name, err)
		}
	}
}

func TestJobStepComplete_afterJobDeletedWhenPhaseMarkedDone(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
	})
	MarkPhaseJobsDone(cluster, PhasePreRollout, semver.MustParse("8.4.0"))
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
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
	c := testClientBuilder().Build()
	uctx := newUpgradeContext(context.Background(), c, cluster, sts, nil)
	step := StepByID(StepDatadirUpgradeCheck)
	if step == nil {
		t.Fatal("step not found")
	}
	if !jobStepComplete(uctx, *step) {
		t.Fatal("expected pre-rollout step complete via phase-done annotation after job deleted")
	}
}

func TestDeleteSucceededJobStepsForPhase_skipsWithoutPhaseDoneMarker(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
	}
	upJob := upgradeCheckJobSucceeded(cluster, "8.4.0")
	c := testClientBuilder().WithObjects(upJob).Build()

	if err := DeleteSucceededJobStepsForPhase(context.Background(), c, cluster, sts, PhasePreRollout); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.Get(context.Background(), types.NamespacedName{Name: JobName(cluster), Namespace: cluster.Namespace}, &batch.Job{}); err != nil {
		t.Fatalf("job must remain until phase-done annotation is persisted: %v", err)
	}
}

func TestDeleteSucceededJobStepsForPhase_keepsFailedJobs(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
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
	active := upgradeCheckJobSucceeded(cluster, "8.4.0")
	active.Status.Succeeded = 0
	active.Status.Active = 1
	active.Status.Conditions = nil
	c := testClientBuilder().WithObjects(active).Build()

	// Phase not complete while upgrade-check Job is still running — nothing deleted.
	if err := DeleteSucceededJobStepsForPhase(context.Background(), c, cluster, sts, PhasePreRollout); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.Get(context.Background(), types.NamespacedName{Name: JobName(cluster), Namespace: cluster.Namespace}, &batch.Job{}); err != nil {
		t.Fatalf("upgrade check job should remain while phase incomplete: %v", err)
	}
}
