/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"errors"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestStorageNeedsExpansion(t *testing.T) {
	oneGi := resource.MustParse("1Gi")
	twoGi := resource.MustParse("2Gi")

	if StorageNeedsExpansion(oneGi, twoGi) {
		t.Fatal("smaller desired should not need expansion")
	}
	if !StorageNeedsExpansion(twoGi, oneGi) {
		t.Fatal("larger desired should need expansion")
	}
	if StorageNeedsExpansion(oneGi, oneGi) {
		t.Fatal("equal sizes should not need expansion")
	}
}

func TestStorageClassAllowsResize(t *testing.T) {
	falseVal := false
	trueVal := true
	if storageClassAllowsResize(nil) {
		t.Fatal("nil storage class")
	}
	if storageClassAllowsResize(&storagev1.StorageClass{}) {
		t.Fatal("unset allowVolumeExpansion")
	}
	if storageClassAllowsResize(&storagev1.StorageClass{AllowVolumeExpansion: &falseVal}) {
		t.Fatal("false allowVolumeExpansion")
	}
	if !storageClassAllowsResize(&storagev1.StorageClass{AllowVolumeExpansion: &trueVal}) {
		t.Fatal("true allowVolumeExpansion")
	}
}

func TestResizeNotSupportedReason(t *testing.T) {
	falseVal := false
	if got := resizeNotSupportedReason("standard", &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "standard"},
		AllowVolumeExpansion: &falseVal,
	}); got == "" {
		t.Fatal("expected reason for SC without expansion")
	}
}

func TestIsPVCResizeForbidden(t *testing.T) {
	err := apierrors.NewForbidden(schema.GroupResource{Resource: "persistentvolumeclaims"}, "data-x",
		errors.New("only dynamically provisioned pvc can be resized"))
	if !isPVCResizeForbidden(err) {
		t.Fatal("expected forbidden resize error")
	}
}
