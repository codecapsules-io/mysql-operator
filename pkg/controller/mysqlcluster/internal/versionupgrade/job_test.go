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
	"strings"
	"testing"

	"github.com/blang/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

func TestUpgradeCheckScript_usesTargetAwareMysqlshCheck(t *testing.T) {
	script := upgradeCheckScript()
	for _, want := range []string{
		"MYSQL_UPGRADE_CHECK_TARGET_VERSION",
		"util.checkForServerUpgrade",
		"--target-version=\"${target}\"",
		"--config-path=\"${config_path}\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("upgrade check script missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "mysqlcheck") {
		t.Fatalf("upgrade check script should not use mysqlcheck:\n%s", script)
	}
}

func TestNewUpgradeCheckJob_mountsClusterConfigMap(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	target := semver.MustParse("8.4.0")
	job, err := newUpgradeCheckJob(cluster, target, options.GetOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantConfigMap := cluster.GetNameForResource(mysqlcluster.ConfigMap)
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("expected one volume, got %d", len(job.Spec.Template.Spec.Volumes))
	}
	vol := job.Spec.Template.Spec.Volumes[0]
	if vol.ConfigMap == nil || vol.ConfigMap.Name != wantConfigMap {
		t.Fatalf("config map volume: got %#v want name %q", vol.ConfigMap, wantConfigMap)
	}

	mounts := job.Spec.Template.Spec.Containers[0].VolumeMounts
	if len(mounts) != 1 || mounts[0].MountPath != constants.ConfMapVolumeMountPath {
		t.Fatalf("unexpected volume mounts: %#v", mounts)
	}

	var gotConfigPath string
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		if e.Name == mysqlUpgradeCheckConfigPath {
			gotConfigPath = e.Value
		}
	}
	if gotConfigPath != upgradeCheckMyCnfPath() {
		t.Fatalf("config path env: got %q want %q", gotConfigPath, upgradeCheckMyCnfPath())
	}
}

func TestNewUpgradeCheckJob_setsTargetVersionEnv(t *testing.T) {
	replicas := int32(1)
	cluster := mysqlcluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Status:     api.MysqlClusterStatus{ReadyNodes: 1},
		Spec: api.MysqlClusterSpec{
			Replicas:     &replicas,
			MysqlVersion: "8.0.34",
			SecretName:   "sec",
		},
	})
	target := semver.MustParse("8.0.34")
	job, err := newUpgradeCheckJob(cluster, target, options.GetOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		if e.Name == mysqlUpgradeCheckTarget && e.Value != target.String() {
			t.Fatalf("target env: got %q want %q", e.Value, target.String())
		}
	}
	if job.Spec.Template.Spec.Containers[0].Args[0] == "" {
		t.Fatal("expected upgrade check script in container args")
	}
}
