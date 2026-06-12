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
	"strings"
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestBuildMysqlConfData_holdsSourceVersionWhenUpgradePathInvalid(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "5.7.35"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: "MY_MYSQL_VERSION", Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = api.SchemeBuilder.AddToScheme(s)
	_ = apps.AddToScheme(s)
	c := fake.NewClientBuilder().WithScheme(s).Build()

	data, err := buildMysqlConfData(c, cluster, sts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(data, "skip-replica-start") {
		t.Fatalf("expected 5.7 my.cnf when upgrade path is invalid, got 8.4 profile:\n%s", data)
	}
}

func TestBuildMysqlConfData_usesTargetForValidUpgrade(t *testing.T) {
	t.Parallel()
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	sts := &apps.StatefulSet{
		Status: apps.StatefulSetStatus{Replicas: 1},
		Spec: apps.StatefulSetSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name: "mysql",
						Env:  []core.EnvVar{{Name: "MY_MYSQL_VERSION", Value: "8.4.0"}},
					}},
				},
			},
		},
	}
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = api.SchemeBuilder.AddToScheme(s)
	_ = apps.AddToScheme(s)
	c := fake.NewClientBuilder().WithScheme(s).Build()

	data, err := buildMysqlConfData(c, cluster, sts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "skip-replica-start") {
		t.Fatalf("expected 8.4 profile for valid upgrade, got:\n%s", data)
	}
	if strings.Contains(data, "default-authentication-plugin") {
		t.Fatalf("expected no 8.0 auth plugin for 8.4 target, got:\n%s", data)
	}
}
