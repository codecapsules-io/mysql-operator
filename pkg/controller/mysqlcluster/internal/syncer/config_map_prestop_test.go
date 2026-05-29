/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlcluster

import (
	"strings"
	"testing"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
)

func TestBuildBashPreStop_usesAppliedVersionDuringUpgrade(t *testing.T) {
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.4.0",
		},
		Status: api.MysqlClusterStatus{
			AppliedMysqlVersion: "8.0.20",
		},
	})

	script := buildBashPreStop(cluster, nil)
	if !strings.Contains(script, "SHOW SLAVE STATUS") {
		t.Fatalf("expected master/slave preStop SQL while applied is 8.0, got: %s", script)
	}
	if strings.Contains(script, "SHOW REPLICA STATUS") {
		t.Fatalf("did not expect replica terminology preStop while applied is 8.0")
	}
}

func TestBuildBashPreStop_usesSpecWhenAppliedMatches(t *testing.T) {
	cluster := mysqlcluster.New(&api.MysqlCluster{
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.4.0",
		},
		Status: api.MysqlClusterStatus{
			AppliedMysqlVersion: "8.4.0",
		},
	})

	script := buildBashPreStop(cluster, nil)
	if !strings.Contains(script, "SHOW REPLICA STATUS") {
		t.Fatalf("expected replica preStop SQL when applied is 8.4, got: %s", script)
	}
}
