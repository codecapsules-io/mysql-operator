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

	"github.com/blang/semver"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func TestObserveDataPlaneVersionSQL_unanimous(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	pod := mysqlReadyPod("c1-mysql-0")
	secret := testOperatorSecret(cluster)

	withMockMysqldVersion("8.0.34-26", func() {
		c := testClientBuilder().WithObjects(secret).Build()
		got, err := ObserveDataPlaneVersionSQL(context.Background(), c, cluster, []core.Pod{pod})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "8.0.34" {
			t.Fatalf("observed version: %s", got)
		}
	})
}

func TestObserveDataPlaneVersionSQL_mixedVersionsHold(t *testing.T) {
	replicas := int32(2)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	pod0 := mysqlReadyPod("c1-mysql-0")
	pod1 := mysqlReadyPod("c1-mysql-1")
	headless := cluster.GetNameForResource(mysqlcluster.HeadlessSVC)
	host0 := mysqldHost(&pod0, headless, cluster.Namespace)
	host1 := mysqldHost(&pod1, headless, cluster.Namespace)
	secret := testOperatorSecret(cluster)

	withMockMysqldVersionPerHost(map[string]string{
		host0: "8.0.34-26",
		host1: "8.4.0-8",
	}, func() {
		c := testClientBuilder().WithObjects(secret).Build()
		_, err := ObserveDataPlaneVersionSQL(context.Background(), c, cluster, []core.Pod{pod0, pod1})
		if !IsHoldRollout(err) {
			t.Fatalf("expected hold when pods disagree: %v", err)
		}
	})
}

func TestObserveDataPlaneVersionSQL_queryFailureHold(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	pod := mysqlReadyPod("c1-mysql-0")
	secret := testOperatorSecret(cluster)

	prev := queryMysqldVersion
	queryMysqldVersion = func(ctx context.Context, user, password, host string) (string, error) {
		return "", &HoldRolloutError{Reason: "connection refused"}
	}
	defer func() { queryMysqldVersion = prev }()

	c := testClientBuilder().WithObjects(secret).Build()
	_, err := ObserveDataPlaneVersionSQL(context.Background(), c, cluster, []core.Pod{pod})
	if !IsHoldRollout(err) {
		t.Fatalf("expected hold on query failure: %v", err)
	}
}

func TestObserveDataPlaneVersionSQL_noReadyPodsHold(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	_, err := ObserveDataPlaneVersionSQL(context.Background(), testClientBuilder().Build(), cluster, nil)
	if !IsHoldRollout(err) {
		t.Fatalf("expected hold with no ready pods: %v", err)
	}
}

func TestParseServerVersion_examples(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"8.0.34-26", "8.0.34"},
		{"8.4.0-8", "8.4.0"},
	}
	for _, tc := range cases {
		v, err := mysqlcluster.ParseServerVersion(tc.in)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.in, err)
		}
		if v.String() != tc.out {
			t.Fatalf("parse %q: got %s want %s", tc.in, v, tc.out)
		}
	}
}

func TestBackfillAppliedVersion_setsApplied(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	pod := mysqlReadyPod("c1-mysql-0")
	secret := testOperatorSecret(cluster)

	withMockMysqldVersion("8.0.34-26", func() {
		c := testClientBuilder().WithObjects(secret).Build()
		updated, err := BackfillAppliedVersion(context.Background(), c, cluster, []core.Pod{pod})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Fatal("expected backfill to update applied")
		}
		if cluster.Status.AppliedMysqlVersion != "8.0.34" {
			t.Fatalf("applied: %q", cluster.Status.AppliedMysqlVersion)
		}
	})
}

func TestBackfillAppliedVersion_skipsWhenAppliedSet(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	updated, err := BackfillAppliedVersion(context.Background(), testClientBuilder().Build(), cluster, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Fatal("should not backfill when applied is set")
	}
}

func TestNeedsAppliedBackfill(t *testing.T) {
	replicas := int32(1)
	withData := mysqlcluster.New(&api.MysqlCluster{
		Status: api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	if !NeedsAppliedBackfill(withData) {
		t.Fatal("expected backfill when applied empty and cluster has data")
	}
	withApplied := mysqlcluster.New(&api.MysqlCluster{
		Status: api.MysqlClusterStatus{AppliedMysqlVersion: "8.0.20"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
			VolumeSpec: api.VolumeSpec{
				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{},
			},
		},
	})
	if NeedsAppliedBackfill(withApplied) {
		t.Fatal("should not backfill when applied is set")
	}
	greenfield := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	if NeedsAppliedBackfill(greenfield) {
		t.Fatal("greenfield without PVC should not need backfill")
	}
}

func TestObserveDataPlaneVersionSQL_ignoresNonReadyPods(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: api.MysqlClusterSpec{
			Replicas:   &replicas,
			SecretName: "sec",
		},
	})
	ready := mysqlReadyPod("c1-mysql-0")
	notReady := core.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-mysql-1"},
		Spec: core.PodSpec{
			Containers: []core.Container{{Name: "mysql"}},
		},
	}
	secret := testOperatorSecret(cluster)

	withMockMysqldVersion("8.4.0-8", func() {
		c := testClientBuilder().WithObjects(secret).Build()
		got, err := ObserveDataPlaneVersionSQL(context.Background(), c, cluster, []core.Pod{ready, notReady})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.EQ(semver.Version{}) {
			t.Fatal("expected version from ready pod only")
		}
	})
}
