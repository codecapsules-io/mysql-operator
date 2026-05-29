/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func TestEnsureVolumeClaimTemplates_volumeModeDefault(t *testing.T) {
	filesystem := core.PersistentVolumeFilesystem
	existing := []core.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{Name: dataVolumeName},
		Spec: core.PersistentVolumeClaimSpec{
			VolumeMode:  &filesystem,
			AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}}
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
					},
				},
			},
		},
	})
	s := &sfsSyncer{cluster: cluster}
	got := s.ensureVolumeClaimTemplates(existing)
	if got[0].Spec.VolumeMode == nil || *got[0].Spec.VolumeMode != core.PersistentVolumeFilesystem {
		t.Fatalf("volumeMode: %v", got[0].Spec.VolumeMode)
	}
}

func TestShouldSetVolumeClaimTemplates(t *testing.T) {
	s := &sfsSyncer{}
	if !s.shouldSetVolumeClaimTemplates(&apps.StatefulSet{}) {
		t.Fatal("new StatefulSet should set volume claim templates")
	}
	existing := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Now(),
		},
		Spec: apps.StatefulSetSpec{
			VolumeClaimTemplates: []core.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: dataVolumeName}}},
		},
	}
	if s.shouldSetVolumeClaimTemplates(existing) {
		t.Fatal("existing StatefulSet with templates must not update volumeClaimTemplates")
	}
}

func TestEnsureVolumeClaimTemplates_storageFromClusterSpec(t *testing.T) {
	existing := []core.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{Name: dataVolumeName},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}}
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("2Gi")},
					},
				},
			},
		},
	})
	s := &sfsSyncer{cluster: cluster}
	got := s.ensureVolumeClaimTemplates(existing)
	gotQty := got[0].Spec.Resources.Requests[core.ResourceStorage]
	if gotQty.Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatalf("storage: %s", gotQty.String())
	}
}

func TestEnsureVolumeClaimTemplates_preservesStorageClassName(t *testing.T) {
	fast := "fast"
	existing := []core.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{Name: dataVolumeName},
		Spec: core.PersistentVolumeClaimSpec{
			StorageClassName: &fast,
			AccessModes:      []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}}
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("2Gi")},
					},
				},
			},
		},
	})
	s := &sfsSyncer{cluster: cluster}
	got := s.ensureVolumeClaimTemplates(existing)
	if got[0].Spec.StorageClassName == nil || *got[0].Spec.StorageClassName != fast {
		t.Fatalf("storageClassName: %v", got[0].Spec.StorageClassName)
	}
}
