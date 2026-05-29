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
	"fmt"
	"strings"

	core "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"

func storageClassAllowsResize(sc *storagev1.StorageClass) bool {
	return sc != nil && sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
}

func resolveStorageClassName(ctx context.Context, c client.Client, pvc *core.PersistentVolumeClaim) (string, error) {
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		return *pvc.Spec.StorageClassName, nil
	}
	list := &storagev1.StorageClassList{}
	if err := c.List(ctx, list); err != nil {
		return "", err
	}
	for i := range list.Items {
		sc := &list.Items[i]
		if sc.Annotations[defaultStorageClassAnnotation] == "true" {
			return sc.Name, nil
		}
	}
	return "", nil
}

func storageClassForPVC(ctx context.Context, c client.Client, pvc *core.PersistentVolumeClaim) (*storagev1.StorageClass, string, error) {
	name, err := resolveStorageClassName(ctx, c, pvc)
	if err != nil {
		return nil, "", err
	}
	if name == "" {
		return nil, "", nil
	}
	sc := &storagev1.StorageClass{}
	if err := c.Get(ctx, client.ObjectKey{Name: name}, sc); err != nil {
		return nil, name, err
	}
	return sc, name, nil
}

func resizeNotSupportedReason(scName string, sc *storagev1.StorageClass) string {
	if scName == "" {
		return "PVC has no storageClassName and no default StorageClass was found; volume expansion requires a StorageClass with allowVolumeExpansion: true"
	}
	if sc == nil {
		return fmt.Sprintf("StorageClass %q not found", scName)
	}
	if !storageClassAllowsResize(sc) {
		return fmt.Sprintf("StorageClass %q does not allow volume expansion (set allowVolumeExpansion: true)", scName)
	}
	return ""
}

func isPVCResizeForbidden(err error) bool {
	if err == nil {
		return false
	}
	if !apierrors.IsForbidden(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resize") || strings.Contains(msg, "resized")
}
