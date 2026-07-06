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
	"testing"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestNeedsDatadirChownInit_whenUpgradingFromAppliedVersion(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20", ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
			Image:        "percona/percona-server:8.4",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	if !NeedsDatadirChownInit(context.Background(), c, cluster) {
		t.Fatal("expected chown init when applied is 8.0 and spec is 8.4")
	}
}

func TestNeedsDatadirChownInit_whenCrashLoopReadyNodesZero(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status: api.MysqlClusterStatus{
			AppliedMysqlVersion: "8.0.34",
			ReadyNodes:          0,
		},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
			Image:        "percona/percona-server:8.4",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	if !NeedsDatadirChownInit(context.Background(), c, cluster) {
		t.Fatal("expected chown init during 8.0→8.4 recovery even when ReadyNodes is zero")
	}
}

func TestNeedsDatadirChownInit_falseWhenAppliedMatchesSpec(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.4.8", ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.8",
			SecretName:   "sec",
			Image:        "percona/percona-server:8.4",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	c := testClientBuilder().Build()
	if NeedsDatadirChownInit(context.Background(), c, cluster) {
		t.Fatal("expected no chown when applied version already matches spec")
	}
}
