/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import batch "k8s.io/api/batch/v1"

// registeredSteps is the catalog of version-upgrade step implementations. Paths that run each step are in upgrade_paths.go.
var registeredSteps = builtinUpgradeSteps()

// UpgradeSteps returns a copy of the registered upgrade step catalog.
func UpgradeSteps() []UpgradeStep {
	out := make([]UpgradeStep, len(registeredSteps))
	copy(out, registeredSteps)
	return out
}

// StepByID returns a registered step or nil.
func StepByID(id string) *UpgradeStep {
	for i := range registeredSteps {
		if registeredSteps[i].ID == id {
			return &registeredSteps[i]
		}
	}
	return nil
}

// StepsForUpgrade returns steps on the current source→target path in the given phase.
func StepsForUpgrade(uctx UpgradeContext, phase Phase) []UpgradeStep {
	return stepsForPhase(uctx, phase)
}

// RegisteredJobTypes returns job-type label values for all Job-based upgrade steps (controller Job watch).
func RegisteredJobTypes() []string {
	seen := make(map[string]struct{})
	var types []string
	for _, s := range registeredSteps {
		if s.Job == nil || s.Job.JobType == "" {
			continue
		}
		if _, ok := seen[s.Job.JobType]; ok {
			continue
		}
		seen[s.Job.JobType] = struct{}{}
		types = append(types, s.Job.JobType)
	}
	return types
}

// IsRegisteredUpgradeJob reports whether the Job was created by a registered upgrade step.
func IsRegisteredUpgradeJob(job *batch.Job) bool {
	if job == nil {
		return false
	}
	jobType := job.Labels["mysql.presslabs.org/job-type"]
	for _, t := range RegisteredJobTypes() {
		if jobType == t {
			return true
		}
	}
	return false
}
