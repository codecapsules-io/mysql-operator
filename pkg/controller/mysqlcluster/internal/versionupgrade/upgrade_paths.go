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
	"github.com/blang/semver"

	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
)

// profileTransition identifies a supported one-step LTS profile upgrade (source line → target line).
type profileTransition struct {
	From string
	To   string
}

// upgradePathSteps lists migration steps for each supported profile transition, in execution order.
// Add a new transition here when supporting another LTS hop; register step implementations in steps_builtin.go.
var upgradePathSteps = map[profileTransition][]string{
	{
		From: mysqlversioning.ProfilePercona57.String(),
		To:   mysqlversioning.ProfilePercona80.String(),
	}: {
		StepDatadirUpgradeCheck,
	},
	{
		From: mysqlversioning.ProfilePercona80.String(),
		To:   mysqlversioning.ProfilePercona84.String(),
	}: {
		StepDatadirUpgradeCheck,
		StepDatadirChown,
		StepAuthPluginMigrate,
	},
}

func stepIDsOnPath(from, to semver.Version) []string {
	fromProfile := mysqlversioning.ProfileFor(from).Name()
	toProfile := mysqlversioning.ProfileFor(to).Name()
	if fromProfile == toProfile {
		return nil
	}
	return upgradePathSteps[profileTransition{From: fromProfile, To: toProfile}]
}

// sourceVersionForStep returns the "from" semver used to resolve the upgrade path for a step.
func sourceVersionForStep(uctx UpgradeContext, stepID string) semver.Version {
	step := StepByID(stepID)
	if step == nil {
		return semver.Version{}
	}
	return step.sourceVersion(uctx)
}

// stepScheduled reports whether the step is listed on the source→target upgrade path.
func stepScheduled(uctx UpgradeContext, stepID string) bool {
	from := sourceVersionForStep(uctx, stepID)
	if from.EQ(semver.Version{}) {
		return false
	}
	for _, id := range stepIDsOnPath(from, uctx.Target) {
		if id == stepID {
			return true
		}
	}
	return false
}

// stepApplicable reports cluster/runtime preconditions for a step already on the upgrade path.
func stepApplicable(uctx UpgradeContext, stepID string) bool {
	step := StepByID(stepID)
	if step == nil {
		return false
	}
	return step.applicable(uctx)
}

// stepRequired is true when the step is on the upgrade path and cluster state allows it to run.
func stepRequired(uctx UpgradeContext, stepID string) bool {
	return stepScheduled(uctx, stepID) && stepApplicable(uctx, stepID)
}

func stepsForPhase(uctx UpgradeContext, phase Phase) []UpgradeStep {
	var out []UpgradeStep
	for _, step := range registeredSteps {
		if step.Phase != phase || !stepRequired(uctx, step.ID) {
			continue
		}
		out = append(out, step)
	}
	return out
}
