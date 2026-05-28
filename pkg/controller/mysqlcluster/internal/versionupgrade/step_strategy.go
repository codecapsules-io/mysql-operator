/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	"github.com/blang/semver"
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
	_ StepStrategy = datadirUpgradeCheckStrategy{}
	_ StepStrategy = datadirChownStrategy{}
	_ StepStrategy = authPluginMigrateStrategy{}
)

type sourceFromUpgradeContext struct{}

func (sourceFromUpgradeContext) SourceVersion(uctx UpgradeContext) semver.Version {
	return uctx.Source
}

type datadirUpgradeCheckStrategy struct {
	source sourceFromUpgradeContext
}

func (s datadirUpgradeCheckStrategy) SourceVersion(uctx UpgradeContext) semver.Version {
	return s.source.SourceVersion(uctx)
}

func (s datadirUpgradeCheckStrategy) Applicable(uctx UpgradeContext) bool {
	if !VersionChangePending(uctx.Cluster, uctx.STS) {
		return false
	}
	return ClusterHasMySQLData(uctx.Cluster, uctx.STS)
}

type datadirChownStrategy struct{}

func (datadirChownStrategy) SourceVersion(uctx UpgradeContext) semver.Version {
	if v := AppliedDataPlaneVersion(uctx.Cluster); !v.EQ(semver.Version{}) {
		return v
	}
	return laggingStatefulSetVersion(uctx.Cluster, uctx.STS)
}

func (s datadirChownStrategy) Applicable(uctx UpgradeContext) bool {
	from := s.SourceVersion(uctx)
	if from.EQ(semver.Version{}) || from.EQ(uctx.Target) {
		return false
	}
	if !HasPersistentDataVolume(uctx.Cluster) || !ClusterHasMySQLData(uctx.Cluster, uctx.STS) {
		return false
	}
	return uctx.Cluster.IsPerconaImage()
}

type authPluginMigrateStrategy struct {
	source sourceFromUpgradeContext
}

func (s authPluginMigrateStrategy) SourceVersion(uctx UpgradeContext) semver.Version {
	return s.source.SourceVersion(uctx)
}

func (s authPluginMigrateStrategy) Applicable(uctx UpgradeContext) bool {
	return VersionChangePending(uctx.Cluster, uctx.STS)
}

func (s *UpgradeStep) sourceVersion(uctx UpgradeContext) semver.Version {
	if s == nil || s.Strategy == nil {
		return semver.Version{}
	}
	return s.Strategy.SourceVersion(uctx)
}

func (s *UpgradeStep) applicable(uctx UpgradeContext) bool {
	if s == nil || s.Strategy == nil {
		return false
	}
	return s.Strategy.Applicable(uctx)
}
