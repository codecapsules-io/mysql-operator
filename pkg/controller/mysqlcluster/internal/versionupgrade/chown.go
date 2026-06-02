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

	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

// DatadirChownInitContainerName is the init container that chowns PVC data for Percona 8.0→8.4 UID migration.
const DatadirChownInitContainerName = "mysql-datadir-chown"

// NeedsDatadirChownInit reports whether the StatefulSet pod template should include the datadir-chown rollout init step.
func NeedsDatadirChownInit(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) bool {
	return NeedsRolloutInit(ctx, c, cluster, sts, StepDatadirChown)
}
