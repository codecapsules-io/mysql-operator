/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"context"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
)

// upgradeDatadirCheckRequired is true when a cross-line upgrade needs the pre-rollout datadir Job.
func upgradeDatadirCheckRequired(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !VersionChangePending(cluster, sts) {
		return false
	}
	source := SourceVersionForUpgrade(cluster, sts)
	if !mysqlversioning.NeedsDatadirUpgradeCheck(source, DesiredSemVer(cluster)) {
		return false
	}
	return ClusterHasMySQLData(cluster, sts)
}

// upgradeCheckJobComplete reports whether the upgrade-check Job succeeded for spec.mysqlVersion.
func upgradeCheckJobComplete(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !upgradeDatadirCheckRequired(cluster, sts) {
		return true
	}
	target := DesiredSemVer(cluster)
	job := &batch.Job{}
	key := types.NamespacedName{Name: JobName(cluster), Namespace: cluster.Namespace}
	if err := c.Get(ctx, key, job); err != nil {
		return false
	}
	if !jobMatchesTarget(job, upgradeCheckTargetLabel, target) {
		return false
	}
	return jobSucceeded(job)
}

func jobMatchesTarget(job *batch.Job, labelKey string, target semver.Version) bool {
	jobTarget := job.Labels[labelKey]
	if jobTarget == "" {
		return false
	}
	parsed, err := semver.Parse(jobTarget)
	if err != nil {
		return jobTarget == target.String()
	}
	return parsed.EQ(target)
}
