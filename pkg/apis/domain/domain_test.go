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

package domain

import (
	"testing"

	core "k8s.io/api/core/v1"
)

func TestComposedStrings(t *testing.T) {
	tests := map[string]string{
		"APIGroup":                         "mysql.presslabs.org",
		"OperatorPrefix":                   "mysql-operator.presslabs.org",
		"BackupsPrefix":                    "backups.mysql.presslabs.org",
		"LabelCluster":                     "mysql.presslabs.org/cluster",
		"LabelServiceType":                 "mysql.presslabs.org/service-type",
		"LabelJobType":                     "mysql.presslabs.org/job-type",
		"LabelUpgradeCheckTargetVersion":   "mysql.presslabs.org/upgrade-check-target-version",
		"LabelAuthMigrateTargetVersion":    "mysql.presslabs.org/auth-migrate-target-version",
		"AnnotationVersion":                "mysql.presslabs.org/version",
		"AnnotationSkipGTIDPurged":         "mysql.presslabs.org/skip-gtid-purged",
		"AnnotationPreRolloutJobsDone":     "mysql.presslabs.org/pre-rollout-jobs-done-version",
		"AnnotationPostRolloutJobsDone":    "mysql.presslabs.org/post-rollout-jobs-done-version",
		"AnnotationResourceDeletionPolicy": "mysql-operator.presslabs.org/resourceDeletionPolicy",
		"FinalizerOrchestrator":            "mysql.presslabs.org/registered-in-orchestrator",
		"FinalizerUser":                    "mysql-operator.presslabs.org/user",
		"FinalizerDatabase":                "mysql-operator.presslabs.org/database",
		"FinalizerRemoteStorageCleanup":    "backups.mysql.presslabs.org/remote-storage-cleanup",
	}

	values := map[string]string{
		"APIGroup":                         APIGroup,
		"OperatorPrefix":                   OperatorPrefix,
		"BackupsPrefix":                    BackupsPrefix,
		"LabelCluster":                     LabelCluster,
		"LabelServiceType":                 LabelServiceType,
		"LabelJobType":                     LabelJobType,
		"LabelUpgradeCheckTargetVersion":   LabelUpgradeCheckTargetVersion,
		"LabelAuthMigrateTargetVersion":    LabelAuthMigrateTargetVersion,
		"AnnotationVersion":                AnnotationVersion,
		"AnnotationSkipGTIDPurged":         AnnotationSkipGTIDPurged,
		"AnnotationPreRolloutJobsDone":     AnnotationPreRolloutJobsDone,
		"AnnotationPostRolloutJobsDone":    AnnotationPostRolloutJobsDone,
		"AnnotationResourceDeletionPolicy": AnnotationResourceDeletionPolicy,
		"FinalizerOrchestrator":            FinalizerOrchestrator,
		"FinalizerUser":                    FinalizerUser,
		"FinalizerDatabase":                FinalizerDatabase,
		"FinalizerRemoteStorageCleanup":    FinalizerRemoteStorageCleanup,
	}

	for name, want := range tests {
		if got := values[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if SchemeGroupVersion.Group != APIGroup {
		t.Errorf("SchemeGroupVersion.Group = %q, want %q", SchemeGroupVersion.Group, APIGroup)
	}
	if SchemeGroupVersion.Version != Version {
		t.Errorf("SchemeGroupVersion.Version = %q, want %q", SchemeGroupVersion.Version, Version)
	}
	if NodeInitializedConditionType != core.PodConditionType(APIGroup+"/NodeInitialized") {
		t.Errorf("NodeInitializedConditionType = %q", NodeInitializedConditionType)
	}
}

func TestIsManagedByMySQL(t *testing.T) {
	if IsManagedByMySQL(nil) {
		t.Fatal("expected false for nil labels")
	}
	if IsManagedByMySQL(map[string]string{"app.kubernetes.io/managed-by": "other"}) {
		t.Fatal("expected false for other manager")
	}
	if !IsManagedByMySQL(map[string]string{"app.kubernetes.io/managed-by": ManagedBy}) {
		t.Fatal("expected true for operator manager")
	}
}
