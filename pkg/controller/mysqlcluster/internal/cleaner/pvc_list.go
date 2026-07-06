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

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func listOwnedClusterPVCs(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster) ([]core.PersistentVolumeClaim, error) {
	pvcs := &core.PersistentVolumeClaimList{}
	opts := &client.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: labels.SelectorFromSet(cluster.GetSelectorLabels()),
	}
	if err := c.List(ctx, pvcs, opts); err != nil {
		return nil, err
	}

	claims := make([]core.PersistentVolumeClaim, 0, len(pvcs.Items))
	for _, claim := range pvcs.Items {
		if !isClusterManagedPVC(claim, cluster) {
			logf.FromContext(ctx).V(1).Info("pvc not managed by cluster", "pvc", claim.Name, "key", cluster)
			continue
		}
		if claim.DeletionTimestamp != nil {
			continue
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

// isClusterManagedPVC reports whether a label-matched PVC belongs to this cluster's data volumes.
// With keepAfterDelete, live PVCs are owned by the StatefulSet rather than the MysqlCluster.
func isClusterManagedPVC(pvc core.PersistentVolumeClaim, cluster *mysqlcluster.MysqlCluster) bool {
	if pvc.Namespace != cluster.Namespace {
		return false
	}
	stsName := cluster.GetNameForResource(mysqlcluster.StatefulSet)
	for _, ref := range pvc.OwnerReferences {
		if ref.Kind == "MysqlCluster" && ref.Name == cluster.Name {
			return true
		}
		if ref.Kind == "StatefulSet" && ref.Name == stsName {
			return true
		}
	}
	return false
}
