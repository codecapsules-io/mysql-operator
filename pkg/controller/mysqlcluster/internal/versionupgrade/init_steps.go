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

// NeedsRolloutInit reports whether the StatefulSet should include the named rollout init step.
func NeedsRolloutInit(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, stepID string) bool {
	return RolloutInitStepRequired(newUpgradeContext(ctx, c, cluster, sts, nil), stepID)
}
