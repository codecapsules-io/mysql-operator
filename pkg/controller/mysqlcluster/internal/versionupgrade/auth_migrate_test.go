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
	"strings"
	"testing"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

func TestEnsureChecked_skipsAuthMigrateOn80Line(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20", ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0.34",
			SecretName:   "sec",
		},
	})
	c := testClientBuilder().Build()
	if err := EnsureChecked(context.Background(), c, cluster, options.GetOptions()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureChecked_skipsAuthMigrateFresh84(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	c := testClientBuilder().Build()
	if err := EnsureChecked(context.Background(), c, cluster, options.GetOptions()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthPluginMigrationRequired_80To84(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
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
	uctx := newUpgradeContext(context.Background(), nil, cluster, sts, options.GetOptions())
	if !stepRequired(uctx, StepAuthPluginMigrate) {
		t.Fatal("expected auth plugin migration for 8.0 -> 8.4 upgrade")
	}
}

func TestAuthMigrateScript_usesMasterSidecarSocketMigration(t *testing.T) {
	script := authMigrateScript()
	for _, want := range []string{
		"MYSQL_AUTH_MIGRATE_TARGET_PLUGIN",
		"TARGET_PLUGIN",
		"MYSQL_AUTH_MIGRATE_POD_HOST",
		"BACKUP_USER",
		"BACKUP_PASSWORD",
		"/auth-migrate",
		"socket-based migration as root",
		"curl -sf",
		"pre-rollout",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("auth migrate script missing %q", want)
		}
	}
	if strings.Contains(script, "mysqladmin --") || strings.Contains(script, "mysql_cmd") {
		t.Fatal("auth migrate must not use TCP mysql client; migration runs on master sidecar via socket")
	}
}

func TestAuthMigrateJobComplete_blocksUntilJobSucceeds(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1},
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
	if JobStepsComplete(context.Background(), c, cluster, sts, PhasePreRollout) {
		t.Fatal("expected auth migrate job to be incomplete before job exists")
	}
}

func TestAuthMigrateJob_usesMasterServiceHost(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	job := newAuthMigrateJob(cluster, semver.MustParse("8.4.0"))
	wantHost := cluster.GetMasterServiceHost()
	var gotHost, gotPod string
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		switch e.Name {
		case mysqlAuthMigrateHost:
			gotHost = e.Value
		case mysqlAuthMigratePodHost:
			gotPod = e.Value
		}
	}
	if gotHost != wantHost {
		t.Fatalf("host: got %q want %q", gotHost, wantHost)
	}
	if gotPod != cluster.GetMasterHost() {
		t.Fatalf("pod host: got %q want %q", gotPod, cluster.GetMasterHost())
	}
}

func TestAuthMigrateJob_defaultTargetPlugin(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	job := newAuthMigrateJob(cluster, semver.MustParse("8.4.0"))
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		if e.Name == mysqlAuthMigrateTargetPlugin && e.Value == defaultAuthMigrateTargetPlugin {
			return
		}
	}
	t.Fatalf("expected %s=%q in job env", mysqlAuthMigrateTargetPlugin, defaultAuthMigrateTargetPlugin)
}

func TestAuthMigrateJob_envUsesOperatedBackupSecret(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "app-secret",
		},
	})
	job := newAuthMigrateJob(cluster, semver.MustParse("8.4.0"))
	env := job.Spec.Template.Spec.Containers[0].Env
	var backupPass core.EnvVar
	for _, e := range env {
		if e.Name == "BACKUP_PASSWORD" {
			backupPass = e
		}
	}
	if backupPass.ValueFrom == nil || backupPass.ValueFrom.SecretKeyRef == nil {
		t.Fatal("BACKUP_PASSWORD must come from operated secret")
	}
	if backupPass.ValueFrom.SecretKeyRef.Name != "c1-mysql-operated" || backupPass.ValueFrom.SecretKeyRef.Key != "BACKUP_PASSWORD" {
		t.Fatalf("unexpected backup secret ref: %+v", backupPass.ValueFrom.SecretKeyRef)
	}
}

func authMigrateJobSucceeded(cluster *mysqlcluster.MysqlCluster, target string) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AuthMigrateJobName(cluster),
			Namespace: cluster.Namespace,
			Labels:    map[string]string{authMigrateTargetLabel: target},
		},
		Status: batch.JobStatus{
			Conditions: []batch.JobCondition{{
				Type:   batch.JobComplete,
				Status: core.ConditionTrue,
			}},
		},
	}
}
