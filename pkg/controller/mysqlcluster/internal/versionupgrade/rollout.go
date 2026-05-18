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
)

// RolloutMySQLVersion is the MySQL version the StatefulSet must run until the upgrade check passes.
// After the check passes it matches spec.mysqlVersion so chown init and the new image roll out together.
func RolloutMySQLVersion(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	desired := DesiredSemVer(cluster)
	if !VersionChangePending(cluster, sts) {
		return desired
	}
	if JobStepsComplete(ctx, c, cluster, sts, PhasePreRollout) {
		return desired
	}
	if v := AppliedDataPlaneVersion(cluster); !v.EQ(semver.Version{}) {
		return v
	}
	if v := laggingStatefulSetVersion(cluster, sts); !v.EQ(semver.Version{}) {
		return v
	}
	return desired
}
