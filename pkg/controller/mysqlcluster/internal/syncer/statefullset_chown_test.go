/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"context"
	"testing"

	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/controller/mysqlcluster/internal/versionupgrade"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

func TestEnsureInitContainersSpec_includesDatadirChownAfterUpgradeCheck(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			Image:        "docker.io/percona/percona-server:8.4",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	sts := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-mysql"},
		Status:     apps.StatefulSetStatus{Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: "MY_MYSQL_VERSION", Value: "8.0.34"}},
					}},
				},
			},
		},
	}
	checkJob := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      versionupgrade.JobName(cluster),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"mysql.presslabs.org/upgrade-check-target-version": "8.4.0",
			},
		},
		Status: batch.JobStatus{
			Succeeded: 1,
			Conditions: []batch.JobCondition{{
				Type:   batch.JobComplete,
				Status: core.ConditionTrue,
			}},
		},
	}
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = api.SchemeBuilder.AddToScheme(sch)
	_ = apps.AddToScheme(sch)
	_ = batch.AddToScheme(sch)
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(checkJob).Build()
	s := &sfsSyncer{
		cluster: cluster,
		client:  c,
		opt:     &options.Options{},
	}
	ctx := context.Background()
	s.rolloutVersion = versionupgrade.RolloutMySQLVersion(ctx, c, cluster, sts)
	inits := s.ensureInitContainersSpec(ctx, sts)
	found := false
	for _, ic := range inits {
		if ic.Name == versionupgrade.DatadirChownInitContainerName && len(ic.Command) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("init containers: %#v", inits)
	}
}
