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
	"fmt"

	"github.com/blang/semver"
	batch "k8s.io/api/batch/v1"
)

func builtinUpgradeSteps() []UpgradeStep {
	return []UpgradeStep{
		datadirUpgradeCheckStep(),
		datadirChownStep(),
		authPluginMigrateStep(),
	}
}

func datadirUpgradeCheckStep() UpgradeStep {
	return UpgradeStep{
		ID:       StepDatadirUpgradeCheck,
		Phase:    PhasePreRollout,
		Strategy: datadirUpgradeCheckStrategy{},
		Job: &JobStepSpec{
			JobType:            JobTypeUpgradeCheck,
			TargetVersionLabel: upgradeCheckTargetLabel,
			JobName:            JobName,
			Build:              buildUpgradeCheckJob,
			WaitReason: func(target semver.Version) string {
				return fmt.Sprintf("waiting for MySQL upgrade check to %s", target)
			},
			FailureLabel: jobFailureMessage,
		},
	}
}

func datadirChownStep() UpgradeStep {
	return UpgradeStep{
		ID:       StepDatadirChown,
		Phase:    PhaseRolloutInit,
		Strategy: datadirChownStrategy{},
		Init: &InitStepSpec{
			ContainerName:       DatadirChownInitContainerName,
			AfterPreRolloutJobs: true,
		},
	}
}

func authPluginMigrateStep() UpgradeStep {
	return UpgradeStep{
		ID:       StepAuthPluginMigrate,
		Phase:    PhasePreRollout,
		Strategy: authPluginMigrateStrategy{},
		Job: &JobStepSpec{
			JobType:            JobTypeAuthMigrate,
			TargetVersionLabel: authMigrateTargetLabel,
			JobName:            AuthMigrateJobName,
			Build:              buildAuthMigrateJob,
			BeforeEnsure:       beforeAuthPluginMigrateJob,
			WaitReason: func(target semver.Version) string {
				return fmt.Sprintf("waiting for MySQL auth plugin migration before rollout to %s", target)
			},
			FailureLabel: authMigrateJobFailureMessage,
		},
	}
}

func buildUpgradeCheckJob(uctx UpgradeContext) (*batch.Job, error) {
	if uctx.STS == nil {
		return nil, fmt.Errorf("statefulset required for upgrade check job")
	}
	return newUpgradeCheckJob(uctx.Cluster, uctx.Target, uctx.Opt, uctx.STS), nil
}

func buildAuthMigrateJob(uctx UpgradeContext) (*batch.Job, error) {
	return newAuthMigrateJob(uctx.Cluster, uctx.Target), nil
}

func beforeAuthPluginMigrateJob(uctx UpgradeContext) error {
	if !ClusterHasRunningMySQL(uctx.Cluster, uctx.STS) {
		return &HoldRolloutError{Reason: "waiting for MySQL master before pre-rollout auth plugin migration"}
	}
	return nil
}

// RolloutInitStepRequired reports whether an init-container step should be injected on the STS template.
func RolloutInitStepRequired(ctx UpgradeContext, stepID string) bool {
	step := StepByID(stepID)
	if step == nil || step.Init == nil || !stepRequired(ctx, stepID) {
		return false
	}
	if step.Init.AfterPreRolloutJobs && !JobStepsComplete(ctx.Ctx, ctx.Client, ctx.Cluster, ctx.STS, PhasePreRollout) {
		return false
	}
	return true
}
