/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func rolloutCompleteOnVersion(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, version semver.Version) bool {
	if sts == nil || cluster.Spec.Replicas == nil {
		return false
	}
	replicas := *cluster.Spec.Replicas
	if replicas == 0 {
		return false
	}
	if !semVerFromStatefulSet(sts).EQ(version) {
		return false
	}
	return int(sts.Status.ReadyReplicas) >= int(replicas)
}

// laggingStatefulSetVersion returns the MySQL version on the STS template when it still lags spec.mysqlVersion.
func laggingStatefulSetVersion(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if sts == nil {
		return semver.Version{}
	}
	desired := DesiredSemVer(cluster)
	if v := semVerFromStatefulSet(sts); !v.EQ(semver.Version{}) && !v.EQ(desired) {
		return v
	}
	return semver.Version{}
}
