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
)

func TestEffectivePVCStorage(t *testing.T) {
	oneGi := resource.MustParse("1Gi")
	twoGi := resource.MustParse("2Gi")

	pvc := &core.PersistentVolumeClaim{
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{core.ResourceStorage: oneGi},
			},
		},
		Status: core.PersistentVolumeClaimStatus{
			Capacity: core.ResourceList{core.ResourceStorage: twoGi},
		},
	}
	got := effectivePVCStorage(pvc)
	if got == nil || got.Cmp(twoGi) != 0 {
		t.Fatalf("expected status capacity 2Gi, got %v", got)
	}
}

func TestMaxDataPVCStorage(t *testing.T) {
	oneGi := resource.MustParse("1Gi")
	threeGi := resource.MustParse("3Gi")

	pvcs := []core.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "data-demo-mysql-0"},
			Spec: core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: oneGi},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "data-demo-mysql-1"},
			Spec: core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: threeGi},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other-pvc"},
			Spec: core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("10Gi")},
				},
			},
		},
	}

	got := maxDataPVCStorage(pvcs, "data-demo-mysql-")
	if got == nil || got.Cmp(threeGi) != 0 {
		t.Fatalf("expected max 3Gi, got %v", got)
	}
}
