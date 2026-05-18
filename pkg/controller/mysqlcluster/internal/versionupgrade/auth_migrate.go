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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

// EnsureAuthMigrated runs a Job on the live master to move mysql_native_password accounts to
// caching_sha2_password when upgrading from the 8.0 line to 8.4+. Returns HoldRolloutError while
// the Job is running. status.appliedMysqlVersion is only advanced after this Job succeeds.
func EnsureAuthMigrated(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, _ *options.Options) error {
	if sts == nil {
		return nil
	}

	desired := DesiredSemVer(cluster)
	if mysqlversioning.ProfileFor(desired).UseMySQL80AuthPlugin() {
		return nil
	}

	if !authPluginMigrationRequired(cluster, sts) {
		return nil
	}

	if authMigrateJobComplete(ctx, c, cluster, sts) {
		return nil
	}

	if !ClusterHasRunningMySQL(cluster, sts) {
		return &HoldRolloutError{Reason: "waiting for MySQL master before auth plugin migration"}
	}

	return ensureAuthMigrateJob(ctx, c, cluster, desired)
}

// authPluginMigrationRequired is true for a pending 8.0-line → 8.4+ upgrade (mysql_native_password removal).
func authPluginMigrationRequired(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !VersionChangePending(cluster, sts) {
		return false
	}
	source := AppliedDataPlaneVersion(cluster)
	if source.EQ(semver.Version{}) {
		source = SourceVersionForUpgrade(cluster, sts)
	}
	return mysqlversioning.NeedsAuthPluginMigration(source, DesiredSemVer(cluster))
}

// authMigrateJobComplete reports whether the auth migrate Job succeeded for the current spec version.
func authMigrateJobComplete(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !authPluginMigrationRequired(cluster, sts) {
		return true
	}
	target := DesiredSemVer(cluster)
	job := &batch.Job{}
	key := types.NamespacedName{Name: AuthMigrateJobName(cluster), Namespace: cluster.Namespace}
	if err := c.Get(ctx, key, job); err != nil {
		return false
	}
	if !jobMatchesTarget(job, authMigrateTargetLabel, target) {
		return false
	}
	return jobSucceeded(job)
}

func ensureAuthMigrateJob(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, target semver.Version) error {
	job := &batch.Job{}
	key := types.NamespacedName{Name: AuthMigrateJobName(cluster), Namespace: cluster.Namespace}
	err := c.Get(ctx, key, job)
	if errors.IsNotFound(err) {
		return createAuthMigrateJob(ctx, c, cluster, target)
	}
	if err != nil {
		return err
	}

	if jobTarget := job.Labels[authMigrateTargetLabel]; jobTarget != "" && !jobMatchesTarget(job, authMigrateTargetLabel, target) {
		if delErr := c.Delete(ctx, job); delErr != nil && !errors.IsNotFound(delErr) {
			return fmt.Errorf("delete stale MySQL auth migrate job: %w", delErr)
		}
		return createAuthMigrateJob(ctx, c, cluster, target)
	}

	if jobSucceeded(job) {
		log.Info("MySQL auth plugin migration completed", "cluster", cluster, "target", target.String())
		return nil
	}

	if failed, msg := jobFailed(job); failed {
		return fmt.Errorf("MySQL auth plugin migration failed for %s: %s", target, msg)
	}

	if job.Status.Failed > 0 {
		return fmt.Errorf("MySQL auth plugin migration blocked: %s", authMigrateJobFailureMessage(job))
	}

	if job.Status.StartTime != nil && job.Status.Active == 0 && job.Status.Succeeded == 0 {
		if time.Since(job.Status.StartTime.Time) > 15*time.Minute {
			return fmt.Errorf("MySQL auth plugin migration blocked: %s", authMigrateJobFailureMessage(job))
		}
	}

	return &HoldRolloutError{Reason: fmt.Sprintf("waiting for MySQL auth plugin migration to %s", target)}
}

func createAuthMigrateJob(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, target semver.Version) error {
	desiredJob := newAuthMigrateJob(cluster, target)
	if createErr := c.Create(ctx, desiredJob); createErr != nil && !errors.IsAlreadyExists(createErr) {
		return fmt.Errorf("create MySQL auth migrate job: %w", createErr)
	}
	log.Info("created MySQL auth migrate job", "cluster", cluster, "target", target.String())
	return &HoldRolloutError{Reason: fmt.Sprintf("waiting for MySQL auth plugin migration to %s", target)}
}
