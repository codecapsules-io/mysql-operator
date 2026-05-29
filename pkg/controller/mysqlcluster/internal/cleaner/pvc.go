/*
Copyright 2018 Platform9 Inc
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
	"strconv"
	"strings"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

const (
	reasonPVCCleanupSuccessful = "SuccessfulPVCCleanup"
	reasonPVCCleanupFailed     = "FailedPVCCleanup"
	messageCleanupSuccessful   = "delete Claim %s in StatefulSet %s successful"
	messageCleanupFailed       = "delete Claim %s in StatefulSet %s failed"
)

var log = logf.Log.WithName("mysqlcluster.pvccleaner")

// PVCCleaner represents an object to clean Pvcs of a MysqlCluster
type PVCCleaner struct {
	cluster  *mysqlcluster.MysqlCluster
	opt      *options.Options
	recorder record.EventRecorder
	client   client.Client
}

// NewPVCCleaner returns a new PVC cleaner object
func NewPVCCleaner(cluster *mysqlcluster.MysqlCluster, opt *options.Options, rec record.EventRecorder, c client.Client) *PVCCleaner {

	return &PVCCleaner{
		cluster:  cluster,
		opt:      opt,
		recorder: rec,
		client:   c,
	}
}

// Run performs cleanup of orphaned pvcs in a Mysql cluster
func (p *PVCCleaner) Run(ctx context.Context) error {
	cluster := p.cluster.Unwrap()

	if cluster.DeletionTimestamp != nil {
		log.V(2).Info("being deleted, no action", "key", p.cluster)
		return nil
	}

	// Find any pvcs with higher ordinal than replicas and delete them
	pvcs, err := listOwnedClusterPVCs(ctx, p.client, p.cluster)
	if err != nil {
		return err
	}

	for _, pvc := range pvcs {
		ord, parseErr := getOrdinal(pvc.Name)
		if parseErr == nil && ord >= *cluster.Spec.Replicas && ord != 0 {
			log.Info("cleaning up PVC", "pvc", pvc.Name, "key", p.cluster)
			if err := p.deletePVC(ctx, &pvc); err != nil {
				return err
			}
		} else if parseErr != nil {
			log.Error(parseErr, "pvc deletion error", "key", p.cluster)
		}
	}

	return nil
}

func (p *PVCCleaner) deletePVC(ctx context.Context, pvc *core.PersistentVolumeClaim) error {
	err := p.client.Delete(ctx, pvc)
	if err != nil {
		p.recorder.Event(p.cluster, core.EventTypeWarning, reasonPVCCleanupFailed,
			fmt.Sprintf(messageCleanupFailed, pvc.Name, p.cluster.Name))
		return err
	}

	p.recorder.Event(p.cluster, core.EventTypeNormal, reasonPVCCleanupSuccessful,
		fmt.Sprintf(messageCleanupSuccessful, pvc.Name, p.cluster.Name))
	return nil
}

func getOrdinal(name string) (int32, error) {
	idx := strings.LastIndexAny(name, "-")
	if idx == -1 {
		return -1, fmt.Errorf("failed to extract ordinal from pvc name: %s", name)
	}

	ordinal, err := strconv.Atoi(name[idx+1:])
	if err != nil {
		log.Error(err, "failed to extract ordinal for pvc", "pvc", name)
		return -1, fmt.Errorf("failed to extract ordinal from pvc name: %s", name)
	}
	return int32(ordinal), nil
}
