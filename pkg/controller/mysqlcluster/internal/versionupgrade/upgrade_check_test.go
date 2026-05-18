/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package versionupgrade

import (
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
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
