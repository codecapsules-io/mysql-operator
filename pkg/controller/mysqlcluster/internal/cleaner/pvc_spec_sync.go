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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

const reasonPVCSpecSynced = "PVCSpecSynced"

var specSyncLog = logf.Log.WithName("mysqlcluster.pvcspecsync")

// PVCSpecSyncer updates cluster spec storage when owned data PVCs are larger than spec.
type PVCSpecSyncer struct {
	cluster  *mysqlcluster.MysqlCluster
	opt      *options.Options
	recorder record.EventRecorder
	client   client.Client
}

// NewPVCSpecSyncer returns a syncer that reconciles cluster spec from live PVC sizes.
func NewPVCSpecSyncer(cluster *mysqlcluster.MysqlCluster, opt *options.Options, rec record.EventRecorder, c client.Client) *PVCSpecSyncer {
	return &PVCSpecSyncer{
		cluster:  cluster,
		opt:      opt,
		recorder: rec,
		client:   c,
	}
}

// Sync updates spec.volumeSpec.persistentVolumeClaim storage when data PVCs exceed the cluster spec.
// Returns true when the in-memory cluster spec was modified and should be persisted.
func (p *PVCSpecSyncer) Sync(ctx context.Context) (bool, error) {
	if p.cluster.Unwrap().DeletionTimestamp != nil {
		return false, nil
	}
	if p.cluster.Spec.VolumeSpec.PersistentVolumeClaim == nil {
		return false, nil
	}

	pvcs, err := listOwnedClusterPVCs(ctx, p.client, p.cluster)
	if err != nil {
		return false, err
	}

	maxPVC := maxDataPVCStorage(pvcs, p.cluster.DataPVCNamePrefix())
	if maxPVC == nil {
		return false, nil
	}

	desired := p.cluster.DesiredDataVolumeStorage()
	if desired != nil && maxPVC.Cmp(*desired) <= 0 {
		return false, nil
	}

	if !p.cluster.SetDesiredDataVolumeStorage(*maxPVC) {
		return false, nil
	}

	from := "unset"
	if desired != nil {
		from = desired.String()
	}
	msg := fmt.Sprintf("updated volumeSpec storage from %s to %s to match expanded PVCs", from, maxPVC.String())
	p.recorder.Event(p.cluster, core.EventTypeNormal, reasonPVCSpecSynced, msg)
	specSyncLog.Info("synced cluster volume spec from PVC storage", "from", from, "to", maxPVC.String(), "key", p.cluster)
	return true, nil
}

func maxDataPVCStorage(pvcs []core.PersistentVolumeClaim, prefix string) *resource.Quantity {
	var max *resource.Quantity
	for i := range pvcs {
		pvc := &pvcs[i]
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		size := effectivePVCStorage(pvc)
		if size == nil {
			continue
		}
		if max == nil || size.Cmp(*max) > 0 {
			copy := size.DeepCopy()
			max = &copy
		}
	}
	return max
}

// effectivePVCStorage returns the larger of PVC spec requests and status capacity.
func effectivePVCStorage(pvc *core.PersistentVolumeClaim) *resource.Quantity {
	var max *resource.Quantity
	if q, ok := pvc.Spec.Resources.Requests[core.ResourceStorage]; ok {
		copy := q.DeepCopy()
		max = &copy
	}
	if q, ok := pvc.Status.Capacity[core.ResourceStorage]; ok {
		if max == nil || q.Cmp(*max) > 0 {
			copy := q.DeepCopy()
			max = &copy
		}
	}
	return max
}
