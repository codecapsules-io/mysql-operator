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
	"context"
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/versionupgrade"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

func TestEnsureInitContainersSpec_includesDatadirChownOnUpgrade(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34", ReadyNodes: 1},
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
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = api.SchemeBuilder.AddToScheme(sch)
	_ = apps.AddToScheme(sch)
	c := fake.NewClientBuilder().WithScheme(sch).Build()
	s := &sfsSyncer{
		cluster: cluster,
		client:  c,
		opt:     &options.Options{},
	}
	ctx := context.Background()
	s.rolloutVersion = versionupgrade.RolloutMySQLVersion(cluster, sts, nil)
	inits := s.ensureInitContainersSpec(ctx)
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
