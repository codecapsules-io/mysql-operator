/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
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

// HoldRolloutError is returned when reconciliation must wait for an upgrade check Job.
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

// EnsureChecked validates a MySQL version change and runs an offline datadir check when required.
// Returns a HoldRolloutError while the check Job is running, or a permanent error when the check fails.
func EnsureChecked(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, opt *options.Options) error {
	sts, err := getStatefulSet(ctx, c, cluster)
	if err != nil {
		return err
	}

	desired := DesiredSemVer(cluster)

	// No spec vs data-plane version gap: nothing to gate (e.g. new cluster where STS already matches
	// spec but status.appliedMysqlVersion is not set yet). Must run before the source-empty check
	// below — otherwise we deadlock: we never sync STS / set applied while holding here forever.
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

	if upgradeCheckJobComplete(ctx, c, cluster, sts) {
		return nil
	}

	if err := mysqlversioning.ValidateUpgradePath(source, desired); err != nil {
		return fmt.Errorf("MySQL version upgrade blocked: %w", err)
	}

	if !upgradeDatadirCheckRequired(cluster, sts) {
		return nil
	}

	return ensureUpgradeCheckJob(ctx, c, cluster, sts, desired, opt)
}

func ensureUpgradeCheckJob(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, target semver.Version, opt *options.Options) error {
	job := &batch.Job{}
	key := types.NamespacedName{Name: JobName(cluster), Namespace: cluster.Namespace}
	err := c.Get(ctx, key, job)
	if errors.IsNotFound(err) {
		return createUpgradeCheckJob(ctx, c, cluster, sts, target, opt)
	}
	if err != nil {
		return err
	}

	if jobTarget := job.Labels[upgradeCheckTargetLabel]; jobTarget != "" && !jobMatchesTarget(job, upgradeCheckTargetLabel, target) {
		if delErr := c.Delete(ctx, job); delErr != nil && !errors.IsNotFound(delErr) {
			return fmt.Errorf("delete stale MySQL upgrade check job: %w", delErr)
		}
		return createUpgradeCheckJob(ctx, c, cluster, sts, target, opt)
	}

	if jobSucceeded(job) {
		log.Info("MySQL upgrade check passed", "cluster", cluster, "target", target.String())
		return nil
	}

	if failed, msg := jobFailed(job); failed {
		return fmt.Errorf("MySQL version upgrade blocked: upgrade check failed for %s: %s", target, msg)
	}

	if job.Status.Failed > 0 {
		return fmt.Errorf("MySQL version upgrade blocked: %s", jobFailureMessage(job))
	}

	if job.Status.StartTime != nil && job.Status.Active == 0 && job.Status.Succeeded == 0 {
		if time.Since(job.Status.StartTime.Time) > 15*time.Minute {
			return fmt.Errorf("MySQL version upgrade blocked: %s", jobFailureMessage(job))
		}
	}

	return &HoldRolloutError{Reason: fmt.Sprintf("waiting for MySQL upgrade check to %s", target)}
}

func createUpgradeCheckJob(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, target semver.Version, opt *options.Options) error {
	desiredJob := newUpgradeCheckJob(cluster, target, opt, sts)
	if createErr := c.Create(ctx, desiredJob); createErr != nil && !errors.IsAlreadyExists(createErr) {
		return fmt.Errorf("create MySQL upgrade check job: %w", createErr)
	}
	log.Info("created MySQL upgrade check job", "cluster", cluster, "target", target.String())
	return &HoldRolloutError{Reason: fmt.Sprintf("waiting for MySQL upgrade check to %s", target)}
}

// ShouldBlockRollout reports whether the cluster controller must not roll out a new MySQL version yet.
func ShouldBlockRollout(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !VersionChangePending(cluster, sts) {
		return false
	}
	return !upgradeCheckJobComplete(ctx, c, cluster, sts)
}

// SyncAppliedVersion sets status.appliedMysqlVersion only after the full rollout succeeds on spec.mysqlVersion,
// including auth plugin migration when upgrading from the 8.0 line to 8.4+.
func SyncAppliedVersion(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, pods []core.Pod) bool {
	desired := DesiredSemVer(cluster)
	if AppliedDataPlaneVersion(cluster).EQ(desired) {
		return false
	}
	if !RolloutComplete(ctx, c, cluster, sts, pods) {
		return false
	}
	if !authMigrateJobComplete(ctx, c, cluster, sts) {
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
