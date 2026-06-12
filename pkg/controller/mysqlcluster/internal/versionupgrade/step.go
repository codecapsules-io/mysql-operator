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
	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

// Step identifiers for built-in upgrade actions. New steps add a constant here, a StepStrategy in step_strategy.go,
// registration in steps_builtin.go, and a path entry in upgrade_paths.go.
const (
	StepDatadirChown = "datadir-chown"
)

// Phase orders when a step runs relative to StatefulSet image rollout.
type Phase int

const (
	// PhaseRolloutInit: Init containers on the target-version pod template.
	PhaseRolloutInit Phase = iota
)

// UpgradeContext carries cluster state for step predicates and init container builders.
type UpgradeContext struct {
	Ctx     context.Context
	Client  client.Client
	Cluster *mysqlcluster.MysqlCluster
	STS     *apps.StatefulSet
	Opt     *options.Options
	Source  semver.Version
	Target  semver.Version
}

func newUpgradeContext(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, opt *options.Options) UpgradeContext {
	source := SourceVersionForUpgrade(cluster, sts)
	return UpgradeContext{
		Ctx:     ctx,
		Client:  c,
		Cluster: cluster,
		STS:     sts,
		Opt:     opt,
		Source:  source,
		Target:  DesiredSemVer(cluster),
	}
}

// UpgradeStep describes one version-transition action (rollout init container).
// Profile transitions list step IDs in upgrade_paths.go; per-step behavior is implemented via Strategy.
type UpgradeStep struct {
	ID       string
	Phase    Phase
	Strategy StepStrategy

	Init *InitStepSpec
}

// InitStepSpec configures an extra init container on the target-version StatefulSet template.
type InitStepSpec struct {
	ContainerName string
}

// InitBuildContext supplies image and version inputs for building rollout init containers.
type InitBuildContext struct {
	UpgradeContext
	RolloutVersion semver.Version
}
