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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestBuiltinSteps_registerStepStrategy(t *testing.T) {
	step := StepByID(StepDatadirChown)
	if step == nil {
		t.Fatalf("step %q not registered", StepDatadirChown)
	}
	if step.Strategy == nil {
		t.Fatalf("step %q missing Strategy", StepDatadirChown)
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
	uctx := newUpgradeContext(context.Background(), nil, cluster, nil)
	step := StepByID(StepDatadirChown)
	got := step.Strategy.SourceVersion(uctx)
	if !got.EQ(semver.MustParse("8.0.34")) {
		t.Fatalf("SourceVersion: got %s want 8.0.34", got)
	}
}
