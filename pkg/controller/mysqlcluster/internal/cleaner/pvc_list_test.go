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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestIsClusterManagedPVC(t *testing.T) {
	trueVar := true
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
	})
	stsName := cluster.GetNameForResource(mysqlcluster.StatefulSet)
	dataPVC := cluster.DataPVCName(0)

	tests := []struct {
		name string
		pvc  core.PersistentVolumeClaim
		want bool
	}{
		{
			name: "mysqlcluster owner reference",
			pvc: core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataPVC,
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{{
						Kind: "MysqlCluster", Name: "demo", Controller: &trueVar,
					}},
				},
			},
			want: true,
		},
		{
			name: "statefulset owner reference",
			pvc: core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataPVC,
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{{
						Kind: "StatefulSet", Name: stsName, Controller: &trueVar,
					}},
				},
			},
			want: true,
		},
		{
			name: "data pvc name without owner reference",
			pvc: core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataPVC,
					Namespace: "default",
				},
			},
			want: false,
		},
		{
			name: "wrong namespace",
			pvc: core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataPVC,
					Namespace: "other",
					OwnerReferences: []metav1.OwnerReference{{
						Kind: "MysqlCluster", Name: "demo",
					}},
				},
			},
			want: false,
		},
		{
			name: "unrelated pvc name and owner",
			pvc: core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-pvc",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{{
						Kind: "StatefulSet", Name: "other-sts",
					}},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClusterManagedPVC(tt.pvc, cluster); got != tt.want {
				t.Fatalf("isClusterManagedPVC() = %v, want %v", got, tt.want)
			}
		})
	}
}
