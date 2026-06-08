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

// Package domain is the single source of truth for the operator API group,
// domain prefixes, labels, annotations, finalizers, and condition types.
//
// When changing the domain, also update:
//   - PROJECT (domain field)
//   - pkg/apis/mysql/v1alpha1/doc.go (+groupName marker)
//   - run `make manifests` to regenerate CRDs and RBAC
//
// Existing clusters retain old metadata keys until migrated; a domain rename is breaking.
package domain

import (
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the kubebuilder resource group name (without domain).
	GroupName = "mysql"
	// Domain is the DNS domain suffix for the API group and metadata keys.
	Domain = "presslabs.org"
	// Version is the API version string.
	Version = "v1alpha1"
)

const (
	// APIGroup is the full Kubernetes API group (mysql.presslabs.org).
	APIGroup = GroupName + "." + Domain
	// OperatorPrefix is the DNS prefix for operator-scoped finalizers and annotations.
	OperatorPrefix = "mysql-operator." + Domain
	// BackupsPrefix is the DNS prefix for backup-related finalizers.
	BackupsPrefix = "backups.mysql." + Domain
	// ManagedBy is the value for app.kubernetes.io/managed-by on operator-owned resources.
	ManagedBy = APIGroup
)

// SchemeGroupVersion is the group version used to register API objects.
var SchemeGroupVersion = schema.GroupVersion{Group: APIGroup, Version: Version}

// Label keys.
const (
	LabelCluster                   = APIGroup + "/cluster"
	LabelServiceType               = APIGroup + "/service-type"
	LabelJobType                   = APIGroup + "/job-type"
	LabelUpgradeCheckMode          = APIGroup + "/upgrade-check-mode"
	LabelUpgradeCheckTargetVersion = APIGroup + "/upgrade-check-target-version"
)

// Label values.
const (
	ServiceTypeMaster         = "master"
	ServiceTypeReadyNodes     = "ready-nodes"
	ServiceTypeReadyReplicas  = "ready-replicas"
	ServiceTypeNamespaceNodes = "namespace-nodes"

	JobTypeUpgradeCheck = "mysql-upgrade-check"

	UpgradeCheckModeOnline  = "online"
	UpgradeCheckModeOffline = "offline"
)

// Annotation keys.
const (
	AnnotationVersion                = APIGroup + "/version"
	AnnotationSkipGTIDPurged         = APIGroup + "/skip-gtid-purged"
	AnnotationPreRolloutJobsDone     = APIGroup + "/pre-rollout-jobs-done-version"
	AnnotationPostRolloutJobsDone    = APIGroup + "/post-rollout-jobs-done-version"
	AnnotationResourceDeletionPolicy = OperatorPrefix + "/resourceDeletionPolicy"
)

// Finalizer names.
const (
	FinalizerOrchestrator         = APIGroup + "/registered-in-orchestrator"
	FinalizerUser                 = OperatorPrefix + "/user"
	FinalizerDatabase             = OperatorPrefix + "/database"
	FinalizerRemoteStorageCleanup = BackupsPrefix + "/remote-storage-cleanup"
)

// NodeInitializedConditionType marks a pod as initialized from MySQL's point of view.
const NodeInitializedConditionType core.PodConditionType = APIGroup + "/NodeInitialized"

// IsManagedByMySQL reports whether labels indicate the resource is managed by this operator.
func IsManagedByMySQL(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	return labels["app.kubernetes.io/managed-by"] == ManagedBy
}
