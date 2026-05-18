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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func TestRolloutComplete_requiresInitContainers(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1"},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
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
					InitContainers: []core.Container{{Name: "init"}, {Name: "mysql-init-only"}},
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: mySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	pod := core.Pod{
		Spec: core.PodSpec{
			InitContainers: []core.Container{{Name: "init"}},
		},
		Status: core.PodStatus{
			InitContainerStatuses: []core.ContainerStatus{{
				Name:  "init",
				State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 0}},
			}},
		},
	}
	c := testClientBuilder().
		WithObjects(upgradeCheckJobSucceeded(cluster, "8.4.0"), authMigrateJobSucceeded(cluster, "8.4.0")).
		Build()
	if RolloutComplete(context.Background(), c, cluster, sts, []core.Pod{pod}) {
		t.Fatal("expected incomplete rollout when not all template init containers succeeded")
	}
}
