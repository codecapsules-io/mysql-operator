/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"testing"

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
