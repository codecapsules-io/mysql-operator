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

	"github.com/blang/semver"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

// NeedsAppliedBackfill reports whether status.appliedMysqlVersion must be populated from SQL.
func NeedsAppliedBackfill(cluster *mysqlcluster.MysqlCluster) bool {
	if !AppliedDataPlaneVersion(cluster).EQ(semver.Version{}) {
		return false
	}
	return ClusterHasMySQLData(cluster)
}

// BackfillAppliedVersion sets status.appliedMysqlVersion from unanimous SELECT VERSION() on ready pods.
func BackfillAppliedVersion(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, pods []core.Pod) (updated bool, err error) {
	if !NeedsAppliedBackfill(cluster) {
		return false, nil
	}
	observed, err := ObserveDataPlaneVersionSQL(ctx, c, cluster, pods)
	if err != nil {
		return false, err
	}
	MarkAppliedVersion(cluster, observed)
	return true, nil
}
