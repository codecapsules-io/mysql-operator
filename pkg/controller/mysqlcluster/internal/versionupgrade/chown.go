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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
)

// DatadirChownInitContainerName is the init container that chowns PVC data for Percona 8.0→8.4 UID migration.
const DatadirChownInitContainerName = "mysql-datadir-chown"

// NeedsDatadirChownInit reports whether the StatefulSet pod template should include mysql-datadir-chown
// for the current upgrade. Uses status.appliedMysqlVersion as the source version (not spec).
func NeedsDatadirChownInit(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	if !upgradeCheckJobComplete(ctx, c, cluster, sts) {
		return false
	}
	from := AppliedDataPlaneVersion(cluster)
	if from.EQ(semver.Version{}) {
		from = laggingStatefulSetVersion(cluster, sts)
	}
	if from.EQ(semver.Version{}) {
		return false
	}
	desired := DesiredSemVer(cluster)
	if from.EQ(desired) {
		return false
	}
	if !HasPersistentDataVolume(cluster) || !ClusterHasMySQLData(cluster, sts) {
		return false
	}
	return mysqlversioning.NeedsDatadirOwnershipMigration(from, desired, cluster.IsPerconaImage())
}
