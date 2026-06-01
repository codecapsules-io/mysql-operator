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
package versionupgrade

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

var log = logf.Log.WithName("versionupgrade")

// HoldRolloutError is returned when reconciliation must wait for an upgrade step.
type HoldRolloutError struct {
	Reason string
}

func (e *HoldRolloutError) Error() string {
	return e.Reason
}

// IsHoldRollout returns true when err blocks rollout until a later reconcile.
func IsHoldRollout(err error) bool {
	_, ok := err.(*HoldRolloutError)
	return ok
}

// EnsureChecked validates a MySQL version change and runs pre-rollout Jobs when required.
func EnsureChecked(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, opt *options.Options) error {
	sts, err := getStatefulSet(ctx, c, cluster)
	if err != nil {
		return err
	}

	if !VersionChangePending(cluster, sts) {
		return nil
	}

	source := SourceVersionForUpgrade(cluster, sts)
	if source.EQ(semver.Version{}) {
		if !ClusterHasMySQLData(cluster, sts) {
			return nil
		}
		return &HoldRolloutError{Reason: "waiting for MySQL data-plane version: cluster must be ready on the current version before upgrading (see status.appliedMysqlVersion)"}
	}

	if err := mysqlversioning.ValidateUpgradePath(source, DesiredSemVer(cluster)); err != nil {
		return fmt.Errorf("MySQL version upgrade blocked: %w", err)
	}

	if JobStepsComplete(ctx, c, cluster, sts, PhasePreRollout) {
		MarkPhaseJobsDone(cluster, PhasePreRollout, DesiredSemVer(cluster))
		if err := DeleteSucceededJobStepsForPhase(ctx, c, cluster, sts, PhasePreRollout); err != nil {
			return fmt.Errorf("delete succeeded pre-rollout upgrade jobs: %w", err)
		}
		return nil
	}

	return EnsureJobSteps(ctx, c, cluster, sts, opt, PhasePreRollout)
}

// EnsurePostRolloutJobs runs Jobs that must succeed after pods are on spec.mysqlVersion.
func EnsurePostRolloutJobs(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, opt *options.Options) error {
	if sts == nil {
		return nil
	}
	return EnsureJobSteps(ctx, c, cluster, sts, opt, PhasePostRollout)
}

// ShouldBlockRollout reports whether the cluster controller must not roll out a new MySQL version yet.
func ShouldBlockRollout(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !VersionChangePending(cluster, sts) {
		return false
	}
	return !JobStepsComplete(ctx, c, cluster, sts, PhasePreRollout)
}

// SyncAppliedVersion sets status.appliedMysqlVersion only after rollout and all post-rollout Jobs succeed.
func SyncAppliedVersion(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, pods []core.Pod) bool {
	desired := DesiredSemVer(cluster)
	if AppliedDataPlaneVersion(cluster).EQ(desired) {
		return false
	}
	if !RolloutComplete(ctx, c, cluster, sts, pods) {
		return false
	}
	if !JobStepsComplete(ctx, c, cluster, sts, PhasePostRollout) {
		return false
	}
	MarkPhaseJobsDone(cluster, PhasePostRollout, desired)
	if err := DeleteSucceededJobStepsForPhase(ctx, c, cluster, sts, PhasePostRollout); err != nil {
		log.Error(err, "failed to delete succeeded post-rollout upgrade jobs", "cluster", cluster)
		return false
	}
	MarkAppliedVersion(cluster, desired)
	return true
}

// GetStatefulSetForRollout loads the cluster StatefulSet if it exists.
func GetStatefulSetForRollout(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster) (*apps.StatefulSet, error) {
	return getStatefulSet(ctx, c, cluster)
}

func getStatefulSet(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster) (*apps.StatefulSet, error) {
	sts := &apps.StatefulSet{}
	key := types.NamespacedName{
		Name:      cluster.GetNameForResource(mysqlcluster.StatefulSet),
		Namespace: cluster.Namespace,
	}
	err := c.Get(ctx, key, sts)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return sts, err
}
