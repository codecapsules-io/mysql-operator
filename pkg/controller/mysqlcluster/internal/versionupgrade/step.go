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
	batch "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

// Step identifiers for built-in upgrade actions. New steps add a constant here, a StepStrategy in step_strategy.go,
// registration in steps_builtin.go, and a path entry in upgrade_paths.go.
const (
	StepDatadirUpgradeCheck = "datadir-upgrade-check"
	StepDatadirChown        = "datadir-chown"
	StepAuthPluginMigrate   = "auth-plugin-migrate"
)

// Phase orders when a step runs relative to StatefulSet image rollout.
type Phase int

const (
	// PhasePreRollout: Jobs that must succeed before the STS may roll out spec.mysqlVersion.
	PhasePreRollout Phase = iota
	// PhaseRolloutInit: Init containers on the target-version pod template (after pre-rollout Jobs).
	PhaseRolloutInit
	// PhasePostRollout: Jobs after pods run spec.mysqlVersion (before status.appliedMysqlVersion advances).
	PhasePostRollout
)

// UpgradeContext carries cluster state for step predicates and Job builders.
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

// UpgradeStep describes one version-transition action (Job and/or rollout init container).
// Profile transitions list step IDs in upgrade_paths.go; per-step behavior is implemented via Strategy.
type UpgradeStep struct {
	ID       string
	Phase    Phase
	Strategy StepStrategy

	Job  *JobStepSpec
	Init *InitStepSpec
}

// JobStepSpec configures a one-shot Kubernetes Job for an upgrade step.
type JobStepSpec struct {
	// JobType is the mysql.presslabs.org/job-type label (Job watch enqueues the cluster).
	JobType string
	// TargetVersionLabel is the label key holding the spec target semver for this Job run.
	TargetVersionLabel string
	JobName            func(*mysqlcluster.MysqlCluster) string
	Build              func(UpgradeContext) (*batch.Job, error)
	// BeforeEnsure runs before creating the Job; return HoldRolloutError to wait, or a permanent error.
	BeforeEnsure func(UpgradeContext) error
	WaitReason   func(target semver.Version) string
	FailureLabel func(*batch.Job) string
}

// InitStepSpec configures an extra init container on the target-version StatefulSet template.
type InitStepSpec struct {
	ContainerName string
	// AfterPreRolloutJobs: do not inject the init container until all PhasePreRollout Jobs succeeded.
	AfterPreRolloutJobs bool
}

// InitBuildContext supplies image and version inputs for building rollout init containers.
type InitBuildContext struct {
	UpgradeContext
	RolloutVersion semver.Version
}
