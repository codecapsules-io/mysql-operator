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

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
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

// UpgradeBlockedError is returned when the requested MySQL version change is permanently invalid
// (downgrade or skipping an LTS line). The cluster remains operational on its current version;
// spec.mysqlVersion must be corrected by the user.
type UpgradeBlockedError struct {
	Reason string
}

func (e *UpgradeBlockedError) Error() string {
	return e.Reason
}

// IsUpgradeBlocked returns true when the requested upgrade path is permanently invalid.
func IsUpgradeBlocked(err error) bool {
	_, ok := err.(*UpgradeBlockedError)
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
		return &UpgradeBlockedError{Reason: fmt.Sprintf("MySQL version upgrade blocked: %s", err.Error())}
	}

	if PreRolloutStepsComplete(ctx, c, cluster, sts) {
		MarkPhaseJobsDone(cluster, PhasePreRollout, DesiredSemVer(cluster))
		return nil
	}

	if err := EnsureJobSteps(ctx, c, cluster, sts, opt, PhasePreRollout); err != nil {
		return err
	}
	if PreRolloutStepsComplete(ctx, c, cluster, sts) {
		MarkPhaseJobsDone(cluster, PhasePreRollout, DesiredSemVer(cluster))
	}
	return nil
}

// PreRolloutStepsComplete reports whether every required pre-rollout Job succeeded for the
// current target (live Job status or the durable phase-done annotation).
func PreRolloutStepsComplete(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	target := DesiredSemVer(cluster)
	if PhaseJobsDoneForTarget(cluster, PhasePreRollout, target) {
		return true
	}
	return PhaseStepsComplete(ctx, c, cluster, sts, PhasePreRollout)
}

// EnsurePostRolloutJobs runs Jobs that must succeed after pods are on spec.mysqlVersion.
func EnsurePostRolloutJobs(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, opt *options.Options) error {
	if sts == nil {
		return nil
	}
	return EnsureJobSteps(ctx, c, cluster, sts, opt, PhasePostRollout)
}

// ShouldBlockRollout reports whether the cluster controller must not roll out a new MySQL version yet.
// Returns true when pre-rollout Jobs are incomplete OR when the upgrade path itself is invalid.
func ShouldBlockRollout(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !VersionChangePending(cluster, sts) {
		return false
	}
	source := SourceVersionForUpgrade(cluster, sts)
	if !source.EQ(semver.Version{}) {
		if err := mysqlversioning.ValidateUpgradePath(source, DesiredSemVer(cluster)); err != nil {
			return true
		}
	}
	return !PreRolloutStepsComplete(ctx, c, cluster, sts)
}

// SyncAppliedVersion reports when rollout and all post-rollout Jobs have succeeded and records the
// post-rollout phase-done marker in memory. The controller must persist that annotation and delete
// succeeded Jobs before calling MarkAppliedVersion.
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
