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
package mysqlcluster

import (
	"testing"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/versionupgrade"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

func TestApplyPodContainerSecurityContext_disabledByDefault(t *testing.T) {
	t.Parallel()

	s := newSecurityContextSyncer(t, "8.0.34", "", &options.Options{})
	pod := core.PodSpec{
		InitContainers: []core.Container{{Name: "init"}},
		Containers:     []core.Container{{Name: containerMysqlName}, {Name: containerSidecarName}},
	}
	s.applyPodContainerSecurityContext(&pod)

	for _, c := range append(pod.InitContainers, pod.Containers...) {
		if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil {
			t.Fatalf("container %q should not have allowPrivilegeEscalation set by default", c.Name)
		}
	}
}

func TestApplyPodContainerSecurityContext_setsFalseWhenEnabled(t *testing.T) {
	t.Parallel()

	s := newSecurityContextSyncer(t, "8.0.34", "", &options.Options{ClusterRestrictPrivilegeEscalation: true})
	pod := core.PodSpec{
		InitContainers: []core.Container{{Name: "init"}},
		Containers:     []core.Container{{Name: containerMysqlName}, {Name: containerSidecarName}},
	}
	s.applyPodContainerSecurityContext(&pod)

	for _, c := range append(pod.InitContainers, pod.Containers...) {
		if c.SecurityContext == nil || c.SecurityContext.AllowPrivilegeEscalation == nil {
			t.Fatalf("container %q missing allowPrivilegeEscalation", c.Name)
		}
		if *c.SecurityContext.AllowPrivilegeEscalation {
			t.Fatalf("container %q: want allowPrivilegeEscalation=false", c.Name)
		}
	}
}

func TestApplyPodContainerSecurityContext_preservesPerContainerOverride(t *testing.T) {
	t.Parallel()

	s := newSecurityContextSyncer(t, "8.0.34", "", &options.Options{ClusterRestrictPrivilegeEscalation: true})
	allowed := true
	pod := core.PodSpec{
		Containers: []core.Container{{
			Name:            "custom",
			SecurityContext: &core.SecurityContext{AllowPrivilegeEscalation: &allowed},
		}},
	}
	s.applyPodContainerSecurityContext(&pod)

	if pod.Containers[0].SecurityContext == nil ||
		pod.Containers[0].SecurityContext.AllowPrivilegeEscalation == nil ||
		!*pod.Containers[0].SecurityContext.AllowPrivilegeEscalation {
		t.Fatalf("per-container allowPrivilegeEscalation should be preserved")
	}
}

func TestApplyPodContainerSecurityContext_setsMysqlRunAsForPercona84(t *testing.T) {
	t.Parallel()

	s := newSecurityContextSyncer(t, "8.4.0", "docker.io/percona/percona-server:8.4", &options.Options{})
	pod := core.PodSpec{
		Containers: []core.Container{{Name: containerMysqlName}},
	}
	s.applyPodContainerSecurityContext(&pod)

	sc := pod.Containers[0].SecurityContext
	if sc == nil || sc.RunAsUser == nil || sc.RunAsGroup == nil {
		t.Fatalf("mysql container missing runAsUser/runAsGroup: %#v", sc)
	}
	if *sc.RunAsUser != 1001 || *sc.RunAsGroup != 1001 {
		t.Fatalf("runAsUser/runAsGroup: want 1001/1001 got %d/%d", *sc.RunAsUser, *sc.RunAsGroup)
	}
}

func TestApplyPodContainerSecurityContext_preservesDatadirChownRunAsRoot(t *testing.T) {
	t.Parallel()

	s := newSecurityContextSyncer(t, "8.4.0", "docker.io/percona/percona-server:8.4", &options.Options{ClusterRestrictPrivilegeEscalation: true})
	root := int64(0)
	pod := core.PodSpec{
		InitContainers: []core.Container{{
			Name:            versionupgrade.DatadirChownInitContainerName,
			SecurityContext: &core.SecurityContext{RunAsUser: &root},
		}},
	}
	s.applyPodContainerSecurityContext(&pod)

	sc := pod.InitContainers[0].SecurityContext
	if sc == nil || sc.RunAsUser == nil || *sc.RunAsUser != 0 {
		t.Fatalf("datadir-chown should keep runAsUser 0, got %#v", sc)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Fatalf("datadir-chown should have allowPrivilegeEscalation=false")
	}
}

func TestSyncFn_doesNotSetAllowPrivilegeEscalationByDefault(t *testing.T) {
	t.Parallel()

	sts := runSecurityContextSyncFn(t, &options.Options{})
	for _, c := range append(sts.Spec.Template.Spec.InitContainers, sts.Spec.Template.Spec.Containers...) {
		if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil {
			t.Fatalf("container %q should not have allowPrivilegeEscalation set by default", c.Name)
		}
	}
}

func TestSyncFn_setsAllowPrivilegeEscalationWhenEnabled(t *testing.T) {
	t.Parallel()

	sts := runSecurityContextSyncFn(t, &options.Options{ClusterRestrictPrivilegeEscalation: true})
	for _, c := range append(sts.Spec.Template.Spec.InitContainers, sts.Spec.Template.Spec.Containers...) {
		if c.SecurityContext == nil || c.SecurityContext.AllowPrivilegeEscalation == nil {
			t.Fatalf("container %q missing allowPrivilegeEscalation", c.Name)
		}
		if *c.SecurityContext.AllowPrivilegeEscalation {
			t.Fatalf("container %q: want allowPrivilegeEscalation=false", c.Name)
		}
	}
}

func newSecurityContextSyncer(t *testing.T, version, image string, opt *options.Options) *sfsSyncer {
	t.Helper()

	replicas := int32(1)
	spec := api.MysqlClusterSpec{
		Replicas:     &replicas,
		MysqlVersion: version,
		SecretName:   "sec",
		VolumeSpec: api.VolumeSpec{
			PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
		},
	}
	if image != "" {
		spec.Image = image
	}

	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec:       spec,
	})

	s := &sfsSyncer{cluster: cluster, opt: opt}
	s.rolloutVersion = cluster.GetMySQLSemVer()
	return s
}

func runSecurityContextSyncFn(t *testing.T, opt *options.Options) *apps.StatefulSet {
	t.Helper()

	s := newSecurityContextSyncer(t, "8.0.34", "", opt)
	sts := &apps.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "c1-mysql"}}
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = api.SchemeBuilder.AddToScheme(sch)
	_ = apps.AddToScheme(sch)
	s.scheme = sch
	s.configMapRevision = "1"
	s.secretRevision = "1"

	if err := s.SyncFn(t.Context(), sts); err != nil {
		t.Fatalf("SyncFn: %v", err)
	}
	return sts
}
