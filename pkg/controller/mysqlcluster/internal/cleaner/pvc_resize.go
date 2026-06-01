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

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

const (
	reasonPVCResizeSuccessful  = "SuccessfulPVCResize"
	reasonPVCResizeFailed      = "FailedPVCResize"
	reasonPVCResizeUnsupported = "PVCResizeUnsupported"
)

var resizeLog = logf.Log.WithName("mysqlcluster.pvcresizer")

// PVCResizer expands cluster data PVCs when spec.volumeSpec storage increases.
type PVCResizer struct {
	cluster  *mysqlcluster.MysqlCluster
	opt      *options.Options
	recorder record.EventRecorder
	client   client.Client
}

// NewPVCResizer returns a resizer for cluster data PVCs.
func NewPVCResizer(cluster *mysqlcluster.MysqlCluster, opt *options.Options, rec record.EventRecorder, c client.Client) *PVCResizer {
	return &PVCResizer{
		cluster:  cluster,
		opt:      opt,
		recorder: rec,
		client:   c,
	}
}

// Run patches owned data PVCs so spec.resources.requests.storage matches the cluster.
func (p *PVCResizer) Run(ctx context.Context) error {
	if p.cluster.Unwrap().DeletionTimestamp != nil {
		return nil
	}

	desired := p.cluster.DesiredDataVolumeStorage()
	if desired == nil {
		return nil
	}

	pvcs, err := listOwnedClusterPVCs(ctx, p.client, p.cluster)
	if err != nil {
		return err
	}

	prefix := p.cluster.DataPVCNamePrefix()
	for i := range pvcs {
		pvc := &pvcs[i]
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		if err := p.resizePVCIfNeeded(ctx, pvc, desired); err != nil {
			return err
		}
	}
	return nil
}

func (p *PVCResizer) resizePVCIfNeeded(ctx context.Context, pvc *core.PersistentVolumeClaim, desired *resource.Quantity) error {
	current, ok := pvc.Spec.Resources.Requests[core.ResourceStorage]
	if !ok {
		return nil
	}
	cmp := desired.Cmp(current)
	if cmp <= 0 {
		if cmp < 0 {
			resizeLog.Info("PVC storage shrink not supported, skipping",
				"pvc", pvc.Name, "current", current.String(), "desired", desired.String(), "key", p.cluster)
		}
		return nil
	}

	updated := pvc.DeepCopy()
	if updated.Spec.Resources.Requests == nil {
		updated.Spec.Resources.Requests = core.ResourceList{}
	}
	newSize := desired.DeepCopy()
	updated.Spec.Resources.Requests[core.ResourceStorage] = newSize

	sc, scName, scErr := storageClassForPVC(ctx, p.client, pvc)
	if scErr != nil {
		return scErr
	}
	if reason := resizeNotSupportedReason(scName, sc); reason != "" {
		p.warnResizeUnsupported(pvc.Name, reason)
		return nil
	}

	if err := p.client.Patch(ctx, updated, client.MergeFrom(pvc)); err != nil {
		if isPVCResizeForbidden(err) {
			p.warnResizeUnsupported(pvc.Name, err.Error())
			return nil
		}
		p.recorder.Event(p.cluster, core.EventTypeWarning, reasonPVCResizeFailed,
			fmt.Sprintf("expand Claim %s to %s failed: %s", pvc.Name, newSize.String(), err))
		return err
	}

	p.recorder.Event(p.cluster, core.EventTypeNormal, reasonPVCResizeSuccessful,
		fmt.Sprintf("expanded Claim %s from %s to %s", pvc.Name, current.String(), newSize.String()))
	resizeLog.Info("expanded PVC storage", "pvc", pvc.Name, "from", current.String(), "to", newSize.String(), "key", p.cluster)
	return nil
}

// StorageNeedsExpansion reports whether desired storage is larger than current.
func StorageNeedsExpansion(desired, current resource.Quantity) bool {
	return desired.Cmp(current) > 0
}

func (p *PVCResizer) warnResizeUnsupported(pvcName, reason string) {
	msg := fmt.Sprintf("cannot expand Claim %s: %s", pvcName, reason)
	p.recorder.Event(p.cluster, core.EventTypeWarning, reasonPVCResizeUnsupported, msg)
	resizeLog.Info(msg, "key", p.cluster)
}
