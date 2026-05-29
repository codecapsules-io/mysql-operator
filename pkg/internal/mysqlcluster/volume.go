/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"fmt"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// DataVolumeName is the StatefulSet volumeClaimTemplate and pod volume name for MySQL data.
const DataVolumeName = "data"

// DesiredDataVolumeStorage returns spec.volumeSpec.persistentVolumeClaim storage requests, if set.
func (c *MysqlCluster) DesiredDataVolumeStorage() *resource.Quantity {
	pvc := c.Spec.VolumeSpec.PersistentVolumeClaim
	if pvc == nil {
		return nil
	}
	qty, ok := pvc.Resources.Requests[core.ResourceStorage]
	if !ok {
		return nil
	}
	return &qty
}

// DataPVCName returns the PVC name for a StatefulSet pod ordinal (e.g. data-demo-mysql-0).
func (c *MysqlCluster) DataPVCName(ordinal int32) string {
	return fmt.Sprintf("data-%s-%d", c.GetNameForResource(StatefulSet), ordinal)
}

// DataPVCNamePrefix returns the prefix for this cluster's data PVCs (e.g. data-demo-mysql-).
func (c *MysqlCluster) DataPVCNamePrefix() string {
	return fmt.Sprintf("data-%s-", c.GetNameForResource(StatefulSet))
}
