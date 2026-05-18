/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"context"
	"testing"

	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

func TestEnsureAuthMigrated_skips80Line(t *testing.T) {
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
	sts := &apps.StatefulSet{Status: apps.StatefulSetStatus{ReadyReplicas: 1, Replicas: 1}}
	c := testClientBuilder().Build()
	if err := EnsureAuthMigrated(context.Background(), c, cluster, sts, options.GetOptions()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureAuthMigrated_skipsFresh84(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
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
	if err := EnsureAuthMigrated(context.Background(), c, cluster, sts, options.GetOptions()); err != nil {
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
	if !authPluginMigrationRequired(cluster, sts) {
		t.Fatal("expected auth plugin migration for 8.0 -> 8.4 upgrade")
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
	if authMigrateJobComplete(context.Background(), c, cluster, sts) {
		t.Fatal("expected auth migrate job to be incomplete before job exists")
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
