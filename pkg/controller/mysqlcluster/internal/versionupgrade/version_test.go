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
	"testing"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

func versionChangePending(cluster *mysqlcluster.MysqlCluster) bool {
	return mysqlversioning.VersionChangePending(
		cluster.DesiredVersion(),
		mysqlcluster.AppliedDataPlaneVersion(cluster),
		ClusterHasMySQLData(cluster),
	)
}

func TestSourceVersionForUpgrade_usesAppliedNotStatefulSetTemplate(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c1",
			Namespace: "default",
		},
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	got := mysqlcluster.SourceVersionForUpgrade(cluster)
	if got.String() != "8.0.20" {
		t.Fatalf("upgrade source version: %s", got)
	}
}

func TestVersionChangePending_appliedBehindSpecDespiteSTS(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	if !versionChangePending(cluster) {
		t.Fatal("expected upgrade pending when applied lags spec")
	}
}

func TestVersionChangePending_legacyClusterWithoutAppliedOrEnv(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	if !versionChangePending(cluster) {
		t.Fatal("expected upgrade pending when applied is unset but cluster has data")
	}
	got := mysqlcluster.SourceVersionForUpgrade(cluster)
	if !got.IsZero() {
		t.Fatalf("source version must be empty without applied: %s", got)
	}
}

func TestVersionChangePending_appliedNewerPatchThanDesiredNotPending(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.34"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0",
			SecretName:   "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	if versionChangePending(cluster) {
		t.Fatal("same profile with applied >= desired must not be pending")
	}
}

func TestVersionChangePending_freshInstallAtDesiredNotPending(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
		},
	})
	if versionChangePending(cluster) {
		t.Fatal("greenfield without MySQL data must not be pending")
	}
}

func TestVersionChangePending_pure(t *testing.T) {
	d := semver.MustParse("8.4.8")
	a := semver.MustParse("8.0.20")
	if !mysqlversioning.VersionChangePending(d, a, true) {
		t.Fatal("expected pending across profiles")
	}
}
