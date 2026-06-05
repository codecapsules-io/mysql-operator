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

// SetDesiredDataVolumeStorage updates spec.volumeSpec.persistentVolumeClaim storage when qty differs.
// Returns true when the spec was changed.
func (c *MysqlCluster) SetDesiredDataVolumeStorage(qty resource.Quantity) bool {
	pvc := c.Spec.VolumeSpec.PersistentVolumeClaim
	if pvc == nil {
		return false
	}
	if pvc.Resources.Requests == nil {
		pvc.Resources.Requests = core.ResourceList{}
	}
	current, ok := pvc.Resources.Requests[core.ResourceStorage]
	if ok && qty.Cmp(current) == 0 {
		return false
	}
	newQty := qty.DeepCopy()
	pvc.Resources.Requests[core.ResourceStorage] = newQty
	return true
}
