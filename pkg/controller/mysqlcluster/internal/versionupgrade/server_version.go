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
	"database/sql"
	"fmt"
	"time"

	"github.com/blang/semver"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"

	_ "github.com/go-sql-driver/mysql"
)

const serverVersionQueryTimeout = 5 * time.Second

// mysqldVersionQuery runs SELECT VERSION() against a single mysqld endpoint.
type mysqldVersionQuery func(ctx context.Context, user, password, host string) (string, error)

var queryMysqldVersion mysqldVersionQuery = defaultQueryMysqldVersion

func defaultQueryMysqldVersion(ctx context.Context, user, password, host string) (string, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s&interpolateParams=true",
		user, password, host, constants.MysqlPort,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	qctx, cancel := context.WithTimeout(ctx, serverVersionQueryTimeout)
	defer cancel()

	var version string
	if err := db.QueryRowContext(qctx, "SELECT VERSION()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

// ObserveDataPlaneVersionSQL returns the unanimous MySQL version reported by SELECT VERSION()
// on every ready mysql pod. Returns HoldRolloutError when observation cannot complete.
func ObserveDataPlaneVersionSQL(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster, pods []core.Pod) (semver.Version, error) {
	ready := readyMysqlPods(pods)
	if len(ready) == 0 {
		return semver.Version{}, &HoldRolloutError{Reason: "waiting for MySQL data-plane version: no ready mysql pods"}
	}

	user, password, err := operatorSQLCredentials(ctx, c, cluster)
	if err != nil {
		return semver.Version{}, err
	}

	headless := cluster.GetNameForResource(mysqlcluster.HeadlessSVC)
	var observed semver.Version
	for i := range ready {
		pod := &ready[i]
		host := mysqldHost(pod, headless, cluster.Namespace)
		raw, err := queryMysqldVersion(ctx, user, password, host)
		if err != nil {
			return semver.Version{}, &HoldRolloutError{
				Reason: fmt.Sprintf("waiting for MySQL data-plane version: query pod %s: %v", pod.Name, err),
			}
		}
		v, err := mysqlcluster.ParseServerVersion(raw)
		if err != nil {
			return semver.Version{}, &HoldRolloutError{
				Reason: fmt.Sprintf("waiting for MySQL data-plane version: parse %q from pod %s: %v", raw, pod.Name, err),
			}
		}
		if observed.EQ(semver.Version{}) {
			observed = v
			continue
		}
		if !observed.EQ(v) {
			return semver.Version{}, &HoldRolloutError{
				Reason: fmt.Sprintf("waiting for MySQL data-plane version: pods disagree (%s vs %s)", observed, v),
			}
		}
	}
	return observed, nil
}

func readyMysqlPods(pods []core.Pod) []core.Pod {
	out := make([]core.Pod, 0, len(pods))
	for i := range pods {
		pod := &pods[i]
		if !podMysqlReady(pod) {
			continue
		}
		out = append(out, *pod)
	}
	return out
}

func podMysqlReady(pod *core.Pod) bool {
	if pod == nil {
		return false
	}
	hasMysql := false
	for _, c := range pod.Spec.Containers {
		if c.Name == "mysql" {
			hasMysql = true
			break
		}
	}
	if !hasMysql {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == core.PodReady && cond.Status == core.ConditionTrue {
			return true
		}
	}
	return false
}

func mysqldHost(pod *core.Pod, headlessSVC, namespace string) string {
	hostname := pod.Spec.Hostname
	if hostname == "" {
		hostname = pod.Name
	}
	return fmt.Sprintf("%s.%s.%s", hostname, headlessSVC, namespace)
}

func operatorSQLCredentials(ctx context.Context, c client.Client, cluster *mysqlcluster.MysqlCluster) (user, password string, err error) {
	secret := &core.Secret{}
	key := types.NamespacedName{
		Name:      cluster.GetNameForResource(mysqlcluster.Secret),
		Namespace: cluster.Namespace,
	}
	if err := c.Get(ctx, key, secret); err != nil {
		return "", "", fmt.Errorf("load operator secret for version query: %w", err)
	}
	user = string(secret.Data["OPERATOR_USER"])
	password = string(secret.Data["OPERATOR_PASSWORD"])
	if user == "" || password == "" {
		return "", "", &HoldRolloutError{Reason: "waiting for MySQL data-plane version: operator credentials not ready"}
	}
	return user, password, nil
}
