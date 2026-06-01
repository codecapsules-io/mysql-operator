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
	"fmt"
	"time"

	"github.com/blang/semver"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

// EnsureJobSteps creates or waits on all required Job steps in the given phase.
func EnsureJobSteps(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, opt *options.Options, phase Phase) error {
	uctx := newUpgradeContext(ctx, c, cluster, sts, opt)
	for _, step := range stepsForPhase(uctx, phase) {
		if step.Job == nil {
			continue
		}
		if err := ensureJobStep(uctx, step); err != nil {
			return err
		}
	}
	return nil
}

// JobStepsComplete reports whether every required Job step in the phase has succeeded for the current target.
func JobStepsComplete(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, phase Phase) bool {
	uctx := newUpgradeContext(ctx, c, cluster, sts, nil)
	for _, step := range stepsForPhase(uctx, phase) {
		if step.Job == nil {
			continue
		}
		if !jobStepComplete(uctx, step) {
			return false
		}
	}
	return true
}

// DeleteSucceededJobStepsForPhase removes succeeded upgrade Jobs (and their pods via foreground
// cascade) once every required step in the phase has completed successfully.
func DeleteSucceededJobStepsForPhase(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, phase Phase) error {
	if sts == nil || !JobStepsComplete(ctx, c, cluster, sts, phase) {
		return nil
	}
	uctx := newUpgradeContext(ctx, c, cluster, sts, nil)
	propagation := metav1.DeletePropagationForeground
	deleteOpts := client.DeleteOptions{PropagationPolicy: &propagation}

	for _, step := range registeredSteps {
		if step.Phase != phase || step.Job == nil {
			continue
		}
		if !stepScheduled(uctx, step.ID) || !jobStepComplete(uctx, step) {
			continue
		}
		name := step.Job.JobName(cluster)
		key := types.NamespacedName{Namespace: cluster.Namespace, Name: name}
		job := &batch.Job{}
		if err := c.Get(ctx, key, job); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}
		if !jobSucceeded(job) {
			continue
		}
		if err := c.Delete(ctx, job, &deleteOpts); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete upgrade job %s/%s: %w", cluster.Namespace, name, err)
		}
		log.Info("deleted succeeded MySQL version upgrade job", "cluster", cluster, "step", step.ID, "job", name, "phase", phase)
	}
	return nil
}

func ensureJobStep(uctx UpgradeContext, step UpgradeStep) error {
	spec := step.Job
	if spec.BeforeEnsure != nil {
		if err := spec.BeforeEnsure(uctx); err != nil {
			return err
		}
	}
	target := uctx.Target
	job := &batch.Job{}
	key := types.NamespacedName{Name: spec.JobName(uctx.Cluster), Namespace: uctx.Cluster.Namespace}
	err := uctx.Client.Get(uctx.Ctx, key, job)
	if errors.IsNotFound(err) {
		return createJobStep(uctx, step)
	}
	if err != nil {
		return err
	}

	if jobTarget := job.Labels[spec.TargetVersionLabel]; jobTarget != "" && !jobMatchesTarget(job, spec.TargetVersionLabel, target) {
		if delErr := uctx.Client.Delete(uctx.Ctx, job); delErr != nil && !errors.IsNotFound(delErr) {
			return fmt.Errorf("delete stale MySQL upgrade job %q: %w", step.ID, delErr)
		}
		return createJobStep(uctx, step)
	}

	if jobSucceeded(job) {
		log.Info("MySQL upgrade step completed", "cluster", uctx.Cluster, "step", step.ID, "target", target.String())
		return nil
	}

	failMsg := spec.FailureLabel
	if failMsg == nil {
		failMsg = func(*batch.Job) string { return step.ID + " job failed" }
	}
	if failed, msg := jobFailed(job); failed {
		return fmt.Errorf("MySQL version upgrade blocked: %s failed for %s: %s", step.ID, target, msg)
	}
	if job.Status.Failed > 0 {
		return fmt.Errorf("MySQL version upgrade blocked: %s", failMsg(job))
	}
	if job.Status.StartTime != nil && job.Status.Active == 0 && job.Status.Succeeded == 0 {
		if time.Since(job.Status.StartTime.Time) > 15*time.Minute {
			return fmt.Errorf("MySQL version upgrade blocked: %s", failMsg(job))
		}
	}

	reason := spec.WaitReason(target)
	return &HoldRolloutError{Reason: reason}
}

func createJobStep(uctx UpgradeContext, step UpgradeStep) error {
	spec := step.Job
	desiredJob, err := spec.Build(uctx)
	if err != nil {
		return fmt.Errorf("build upgrade job %q: %w", step.ID, err)
	}
	if createErr := uctx.Client.Create(uctx.Ctx, desiredJob); createErr != nil && !errors.IsAlreadyExists(createErr) {
		return fmt.Errorf("create MySQL upgrade job %q: %w", step.ID, createErr)
	}
	log.Info("created MySQL upgrade job", "cluster", uctx.Cluster, "step", step.ID, "target", uctx.Target.String())
	return &HoldRolloutError{Reason: spec.WaitReason(uctx.Target)}
}

func jobStepComplete(uctx UpgradeContext, step UpgradeStep) bool {
	if step.Job == nil || !stepRequired(uctx, step.ID) {
		return true
	}
	spec := step.Job
	target := uctx.Target
	job := &batch.Job{}
	key := types.NamespacedName{Name: spec.JobName(uctx.Cluster), Namespace: uctx.Cluster.Namespace}
	if err := uctx.Client.Get(uctx.Ctx, key, job); err != nil {
		if errors.IsNotFound(err) && PhaseJobsDoneForTarget(uctx.Cluster, step.Phase, target) {
			return true
		}
		return false
	}
	if !jobMatchesTarget(job, spec.TargetVersionLabel, target) {
		return false
	}
	return jobSucceeded(job)
}

func jobMatchesTarget(job *batch.Job, labelKey string, target semver.Version) bool {
	jobTarget := job.Labels[labelKey]
	if jobTarget == "" {
		return false
	}
	parsed, err := semver.Parse(jobTarget)
	if err != nil {
		return jobTarget == target.String()
	}
	return parsed.EQ(target)
}
