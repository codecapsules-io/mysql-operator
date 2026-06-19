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
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

// RolloutInitStepsPending reports whether required pre-rollout init steps (e.g. datadir-chown)
// are not yet complete on all replicas. While pending, RolloutMySQLVersion stays on the source version.
func RolloutInitStepsPending(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, pods []core.Pod) bool {
	if !VersionChangePending(cluster) {
		return false
	}
	uctx := newUpgradeContext(ctx, c, cluster, nil)
	if uctx.Source.EQ(uctx.Target) {
		return false
	}
	return rolloutInitStepsPending(uctx, sts, pods)
}

func rolloutInitStepsPending(uctx UpgradeContext, sts *apps.StatefulSet, pods []core.Pod) bool {
	for _, stepID := range stepIDsOnPath(uctx.Source, uctx.Target) {
		step := StepByID(stepID)
		if step == nil || step.Init == nil || step.Phase != PhaseRolloutInit {
			continue
		}
		if !stepRequired(uctx, stepID) {
			continue
		}
		if !rolloutInitStepComplete(sts, pods, step.Init.ContainerName, uctx.Cluster) {
			return true
		}
	}
	return false
}

func rolloutInitStepComplete(sts *apps.StatefulSet, pods []core.Pod, initName string, cluster *mysqlcluster.MysqlCluster) bool {
	if sts == nil || cluster.Spec.Replicas == nil {
		return false
	}
	replicas := *cluster.Spec.Replicas
	if replicas == 0 {
		return true
	}
	if !initContainerOnStatefulSetTemplate(sts, initName) {
		return false
	}
	if len(pods) == 0 {
		return false
	}
	templateInits := []core.Container{{Name: initName}}
	var ready int32
	for i := range pods {
		pod := &pods[i]
		if !podHasTemplateInitContainers(pod, templateInits) {
			continue
		}
		if !allInitContainersSucceeded(pod, templateInits) {
			continue
		}
		ready++
	}
	return ready >= replicas
}

func initContainerOnStatefulSetTemplate(sts *apps.StatefulSet, name string) bool {
	for _, ic := range sts.Spec.Template.Spec.InitContainers {
		if ic.Name == name && len(ic.Command) > 0 {
			return true
		}
	}
	return false
}
