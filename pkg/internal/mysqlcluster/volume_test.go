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

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
)

func TestDataPVCName(t *testing.T) {
	cluster := New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
	})
	if got := cluster.DataPVCName(2); got != "data-demo-mysql-2" {
		t.Fatalf("DataPVCName: %s", got)
	}
	if got := cluster.DataPVCNamePrefix(); got != "data-demo-mysql-" {
		t.Fatalf("DataPVCNamePrefix: %s", got)
	}
}

func TestDesiredDataVolumeStorage(t *testing.T) {
	cluster := New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: resource.MustParse("5Gi"),
						},
					},
				},
			},
		},
	})
	qty := cluster.DesiredDataVolumeStorage()
	if qty == nil || qty.Cmp(resource.MustParse("5Gi")) != 0 {
		t.Fatalf("DesiredDataVolumeStorage: %v", qty)
	}
}
