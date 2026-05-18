/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"context"

	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

// DatadirChownInitContainerName is the init container that chowns PVC data for Percona 8.0→8.4 UID migration.
const DatadirChownInitContainerName = "mysql-datadir-chown"

// NeedsDatadirChownInit reports whether the StatefulSet pod template should include the datadir-chown rollout init step.
func NeedsDatadirChownInit(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	return NeedsRolloutInit(ctx, c, cluster, sts, StepDatadirChown)
}
