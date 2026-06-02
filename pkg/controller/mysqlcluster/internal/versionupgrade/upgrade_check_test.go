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
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

func upgradeCheckJobSucceeded(cluster *mysqlcluster.MysqlCluster, target string) *batch.Job {
	return &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(cluster),
			Namespace: cluster.Namespace,
			Labels:    map[string]string{upgradeCheckTargetLabel: target},
		},
		Status: batch.JobStatus{
			Succeeded: 1,
			Conditions: []batch.JobCondition{{
				Type:   batch.JobComplete,
				Status: core.ConditionTrue,
			}},
		},
	}
}
