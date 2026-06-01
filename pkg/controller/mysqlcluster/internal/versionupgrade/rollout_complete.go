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

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

// RolloutComplete is true when spec.mysqlVersion is fully running: StatefulSet template matches spec,
// every replica is ready, any required pre-upgrade check has passed, and every init container on the
// current pod template has completed successfully on each pod.
func RolloutComplete(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet, pods []core.Pod) bool {
	if sts == nil || cluster.Spec.Replicas == nil {
		return false
	}
	replicas := *cluster.Spec.Replicas
	if replicas == 0 {
		return false
	}
	desired := DesiredSemVer(cluster)
	if !rolloutCompleteOnVersion(cluster, sts, desired) {
		return false
	}
	if !JobStepsComplete(ctx, c, cluster, sts, PhasePreRollout) {
		return false
	}
	return podTemplateInitContainersSucceeded(sts, pods, replicas)
}

func podTemplateInitContainersSucceeded(sts *apps.StatefulSet, pods []core.Pod, replicas int32) bool {
	templateInits := sts.Spec.Template.Spec.InitContainers
	if len(templateInits) == 0 {
		return int32(len(pods)) >= replicas
	}
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

func podHasTemplateInitContainers(pod *core.Pod, templateInits []core.Container) bool {
	for _, want := range templateInits {
		found := false
		for _, have := range pod.Spec.InitContainers {
			if have.Name == want.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func allInitContainersSucceeded(pod *core.Pod, templateInits []core.Container) bool {
	for _, ic := range templateInits {
		if !initContainerSucceeded(pod, ic.Name) {
			return false
		}
	}
	return true
}

func initContainerSucceeded(pod *core.Pod, name string) bool {
	for _, s := range pod.Status.InitContainerStatuses {
		if s.Name != name {
			continue
		}
		if s.State.Terminated != nil {
			return s.State.Terminated.ExitCode == 0
		}
		return false
	}
	return false
}
