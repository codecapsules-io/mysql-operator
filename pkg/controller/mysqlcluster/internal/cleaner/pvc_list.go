/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"context"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
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
		if !isOwnedBy(claim, cluster.Unwrap()) {
			logf.FromContext(ctx).V(1).Info("pvc not owned by cluster", "pvc", claim.Name, "key", cluster)
			continue
		}
		if claim.DeletionTimestamp != nil {
			continue
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

func isOwnedBy(pvc core.PersistentVolumeClaim, cluster *api.MysqlCluster) bool {
	if pvc.Namespace != cluster.Namespace {
		return false
	}
	for _, ref := range pvc.OwnerReferences {
		if ref.Kind == "MysqlCluster" && ref.Name == cluster.Name {
			return true
		}
	}
	return false
}
