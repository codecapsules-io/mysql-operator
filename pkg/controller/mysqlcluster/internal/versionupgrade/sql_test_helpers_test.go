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
	"sync"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
)

// withMockMysqldVersion runs fn with queryMysqldVersion returning version for every host.
func withMockMysqldVersion(version string, fn func()) {
	prev := queryMysqldVersion
	queryMysqldVersion = func(ctx context.Context, user, password, host string) (string, error) {
		return version, nil
	}
	defer func() { queryMysqldVersion = prev }()
	fn()
}

// testOperatorSecret returns the operated secret used for SQL version queries in tests.
func testOperatorSecret(cluster *mysqlcluster.MysqlCluster) *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.Secret),
			Namespace: cluster.Namespace,
		},
		Data: map[string][]byte{
			"OPERATOR_USER":     []byte("op"),
			"OPERATOR_PASSWORD": []byte("pass"),
		},
	}
}

// mysqlReadyPod returns a pod that passes readyMysqlPods for SQL observation tests.
func mysqlReadyPod(name string) core.Pod {
	return core.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: core.PodSpec{
			Hostname: name,
			Containers: []core.Container{{
				Name: "mysql",
			}},
		},
		Status: core.PodStatus{
			Conditions: []core.PodCondition{{
				Type:   core.PodReady,
				Status: core.ConditionTrue,
			}},
		},
	}
}

// withMockMysqldVersionPerHost runs fn with per-host version responses.
func withMockMysqldVersionPerHost(versions map[string]string, fn func()) {
	prev := queryMysqldVersion
	var mu sync.Mutex
	queryMysqldVersion = func(ctx context.Context, user, password, host string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if v, ok := versions[host]; ok {
			return v, nil
		}
		return "", &HoldRolloutError{Reason: "unexpected host " + host}
	}
	defer func() { queryMysqldVersion = prev }()
	fn()
}
