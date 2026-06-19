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
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

// StepStrategy encapsulates per-step upgrade path behavior (strategy pattern).
// Each registered UpgradeStep supplies a concrete implementation instead of package-level switches.
type StepStrategy interface {
	// SourceVersion returns the "from" semver used to resolve profile transitions for this step.
	SourceVersion(uctx UpgradeContext) semver.Version
	// Applicable reports runtime preconditions for running the step on the current cluster.
	Applicable(uctx UpgradeContext) bool
}

var (
	_ StepStrategy = datadirChownStrategy{}
)

type datadirChownStrategy struct{}

func (datadirChownStrategy) SourceVersion(uctx UpgradeContext) semver.Version {
	return uctx.Source
}

func (s datadirChownStrategy) Applicable(uctx UpgradeContext) bool {
	from := s.SourceVersion(uctx)
	if from.IsZero() || from.EQ(uctx.Target) {
		return false
	}
	if !HasPersistentDataVolume(uctx.Cluster) {
		return false
	}
	return uctx.Cluster.IsPerconaImage()
}

func (s *UpgradeStep) sourceVersion(uctx UpgradeContext) semver.Version {
	if s == nil || s.Strategy == nil {
		return semver.Zero
	}
	return s.Strategy.SourceVersion(uctx)
}

func (s *UpgradeStep) applicable(uctx UpgradeContext) bool {
	if s == nil || s.Strategy == nil {
		return false
	}
	return s.Strategy.Applicable(uctx)
}
