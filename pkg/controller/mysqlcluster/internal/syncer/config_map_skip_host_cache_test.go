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
	icluster "github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildMysqlConfData_skipHostCacheByVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version      string
		wantSkipHost bool
	}{
		{"8.0.20", true},
		{"8.0.29", true},
		{"8.0.30", false},
		{"8.4.8", false},
		{"9.7.0", false},
	} {
		tc := tc
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			c := icluster.New(&api.MysqlCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
				Spec: api.MysqlClusterSpec{
					MysqlVersion: tc.version,
					SecretName:   "sec",
				},
			})
			data, err := buildMysqlConfData(c)
			if err != nil {
				t.Fatal(err)
			}
			has := strings.Contains(data, "skip-host-cache")
			if has != tc.wantSkipHost {
				t.Fatalf("skip-host-cache present=%v want present=%v", has, tc.wantSkipHost)
			}
			if !strings.Contains(data, "skip-name-resolve") {
				t.Fatal("expected skip-name-resolve in generated my.cnf")
			}
		})
	}
}
