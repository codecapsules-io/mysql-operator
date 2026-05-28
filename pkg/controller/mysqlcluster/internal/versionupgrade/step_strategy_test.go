/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"context"
	"testing"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func TestBuiltinSteps_registerStepStrategy(t *testing.T) {
	for _, id := range []string{StepDatadirUpgradeCheck, StepDatadirChown, StepAuthPluginMigrate} {
		step := StepByID(id)
		if step == nil {
			t.Fatalf("step %q not registered", id)
		}
		if step.Strategy == nil {
			t.Fatalf("step %q missing Strategy", id)
		}
	}
}

func TestDatadirChownStrategy_sourceVersionPrefersApplied(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
		},
	})
	sts := &apps.StatefulSet{
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
	uctx := newUpgradeContext(context.Background(), nil, cluster, sts, nil)
	step := StepByID(StepDatadirChown)
	got := step.Strategy.SourceVersion(uctx)
	if !got.EQ(semver.MustParse("8.0.34")) {
		t.Fatalf("SourceVersion: got %s want 8.0.34", got)
	}
}

func TestDatadirUpgradeCheckStrategy_usesUpgradeContextSource(t *testing.T) {
	uctx := UpgradeContext{Source: semver.MustParse("8.0.20")}
	s := datadirUpgradeCheckStrategy{}
	if !s.SourceVersion(uctx).EQ(uctx.Source) {
		t.Fatalf("expected SourceVersion to delegate to uctx.Source")
	}
}
