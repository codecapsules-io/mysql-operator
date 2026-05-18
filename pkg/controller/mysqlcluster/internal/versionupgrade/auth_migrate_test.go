/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
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

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
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

func TestAuthMigrateScript_usesServerQuotedAlterStatements(t *testing.T) {
	script := authMigrateScript()
	if strings.Contains(script, `IFS=$'\t' read`) {
		t.Fatal("auth migrate must not split mysql batch output on tabs in shell")
	}
	for _, want := range []string{
		"IDENTIFIED WITH caching_sha2_password BY",
		"RETAIN CURRENT PASSWORD",
		"sys_replication",
		"init-file",
		"MYSQL_APP_USER",
		"MYSQL_ROOT_PASSWORD", "OPERATOR_USER", "--protocol=TCP",
		"pre-rollout", "ORDER BY (user =", "one session",
		"user <>",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("auth migrate script missing %q", want)
		}
	}
	if strings.Contains(script, "mysqladmin --") || strings.Contains(script, "mysqladmin_cmd") {
		t.Fatal("auth migrate must not invoke mysqladmin (ping does not verify credentials)")
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
		case "MYSQL_AUTH_MIGRATE_POD_HOST":
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

func TestAuthMigrateJob_envUsesClusterRootSecret(t *testing.T) {
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
	var rootUser, rootPass core.EnvVar
	for _, e := range env {
		switch e.Name {
		case "MYSQL_ROOT_USER":
			rootUser = e
		case "MYSQL_ROOT_PASSWORD":
			rootPass = e
		}
	}
	if rootUser.Value != "root" {
		t.Fatalf("MYSQL_ROOT_USER: got %q", rootUser.Value)
	}
	if rootPass.ValueFrom == nil || rootPass.ValueFrom.SecretKeyRef == nil {
		t.Fatal("MYSQL_ROOT_PASSWORD must come from cluster secret")
	}
	if rootPass.ValueFrom.SecretKeyRef.Name != "app-secret" || rootPass.ValueFrom.SecretKeyRef.Key != "ROOT_PASSWORD" {
		t.Fatalf("unexpected root secret ref: %+v", rootPass.ValueFrom.SecretKeyRef)
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
