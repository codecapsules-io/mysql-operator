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
package mysqlcluster

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

func TestDesiredVersion_specThenDefault(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	c := New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	if got := c.DesiredVersion().String(); got != "8.4.0" {
		t.Fatalf("desired: %s", got)
	}

	c.Spec.MysqlVersion = ""
	if got := c.DesiredVersion().String(); got != constants.MySQLDefaultVersion {
		t.Fatalf("empty spec should use default: %s", got)
	}
}

func TestSourceVersionForUpgrade_usesAppliedNotStatefulSetTemplate(t *testing.T) {
	replicas := int32(1)
	cluster := New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: MySQLVersionEnv, Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	got := SourceVersionForUpgrade(cluster, sts)
	if got.String() != "8.0.20" {
		t.Fatalf("upgrade source version: %s", got)
	}
}

func TestEffectiveVersion_usesLaggingSTSBeforeDesired(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: MySQLVersionEnv, Value: "8.0.20"}},
					}},
				},
			},
		},
	}
	got := cluster.EffectiveVersion(sts)
	if got.String() != "8.0.20" {
		t.Fatalf("effective with lagging STS: %s", got)
	}
}

func TestEffectiveVersion_fallsBackToDesiredOnFreshInstall(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	got := cluster.EffectiveVersion(nil)
	if got.String() != "8.4.0" {
		t.Fatalf("fresh install effective: %s", got)
	}
}

func TestSemVerFromPod_prefersPodEnv(t *testing.T) {
	t.Parallel()
	pod := &core.Pod{
		Spec: core.PodSpec{
			Containers: []core.Container{{
				Name: "mysql",
				Env:  []core.EnvVar{{Name: MySQLVersionEnv, Value: "8.4.8"}},
			}},
		},
	}
	got := SemVerFromPod(pod)
	if got.String() != "8.4.8" {
		t.Fatalf("pod version: %s", got)
	}
}
