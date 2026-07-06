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

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func TestSemVerFromStatefulSet_legacyImageWithoutEnv(t *testing.T) {
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name:  "mysql",
						Image: "docker.io/percona/percona-server:8.0.34",
					}},
				},
			},
		},
	}
	got := SemVerFromStatefulSet(sts)
	if got.String() != "8.0.34" {
		t.Fatalf("legacy STS version: %s", got)
	}
}

func TestSemVerFromStatefulSet_perconaBuildSuffix(t *testing.T) {
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name:  "mysql",
						Image: "docker.io/percona/percona-server:8.0.34-26",
					}},
				},
			},
		},
	}
	got := SemVerFromStatefulSet(sts)
	if got.String() != "8.0.34" {
		t.Fatalf("percona build suffix: %s", got)
	}
}

func TestLaggingStatefulSetVersion_legacyImageWithoutEnv(t *testing.T) {
	replicas := int32(1)
	cluster := New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	sts := &apps.StatefulSet{
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name:  "mysql",
						Image: "docker.io/percona/percona-server:8.0.34",
					}},
				},
			},
		},
	}
	got := LaggingStatefulSetVersion(cluster, sts)
	if got.String() != "8.0.34" {
		t.Fatalf("lagging legacy STS version: %s", got)
	}
}
