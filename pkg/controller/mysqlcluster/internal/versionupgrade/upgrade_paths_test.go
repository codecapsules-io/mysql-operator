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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
)

func TestStepIDsOnPath_sameProfile(t *testing.T) {
	v := semver.MustParse("8.0.20")
	if got := stepIDsOnPath(v, semver.MustParse("8.0.34")); got != nil {
		t.Fatalf("patch bump should have no path steps, got %v", got)
	}
}

func TestStepIDsOnPath_80To84(t *testing.T) {
	from := semver.MustParse("8.0.34")
	to := semver.MustParse("8.4.0")
	got := stepIDsOnPath(from, to)
	want := []string{StepDatadirUpgradeCheck, StepDatadirChown, StepAuthPluginMigrate}
	if len(got) != len(want) {
		t.Fatalf("steps: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestStepIDsOnPath_57To80(t *testing.T) {
	got := stepIDsOnPath(semver.MustParse("5.7.44"), semver.MustParse("8.0.34"))
	if len(got) != 1 || got[0] != StepDatadirUpgradeCheck {
		t.Fatalf("57→80: got %v", got)
	}
}

func TestStepIDsOnPath_unmappedTransition(t *testing.T) {
	// Skipping an LTS line is blocked by ValidateUpgradePath; no steps are scheduled.
	got := stepIDsOnPath(semver.MustParse("5.7.44"), semver.MustParse("8.4.0"))
	if got != nil {
		t.Fatalf("unmapped transition should have no steps, got %v", got)
	}
}

func TestStepRequired_authMigrate80To84(t *testing.T) {
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
	uctx := newUpgradeContext(context.Background(), nil, cluster, sts, nil)
	if !stepRequired(uctx, StepAuthPluginMigrate) {
		t.Fatalf("source=%s target=%s scheduled=%v applicable=%v",
			uctx.Source, uctx.Target,
			stepScheduled(uctx, StepAuthPluginMigrate),
			stepApplicable(uctx, StepAuthPluginMigrate))
	}
}

func TestUpgradePathSteps_useProfileNames(t *testing.T) {
	key := profileTransition{
		From: mysqlversioning.ProfilePercona80.String(),
		To:   mysqlversioning.ProfilePercona84.String(),
	}
	steps, ok := upgradePathSteps[key]
	if !ok {
		t.Fatal("expected 8.0→8.4 path in map")
	}
	if len(steps) < 3 {
		t.Fatalf("8.0→8.4 steps: %v", steps)
	}
}
