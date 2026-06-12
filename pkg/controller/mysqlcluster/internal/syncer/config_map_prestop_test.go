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
package mysqlcluster

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"

	"github.com/codecapsules-io/mysql-operator/pkg/util/mysqlversion"
)

func TestBuildBashPreStop_branchesOnMySQLVersion(t *testing.T) {
	script := buildBashPreStop()

	required := []string{
		"MY_MYSQL_VERSION",
		"use_source_replica_terms",
		"prestop_mysql_use_source_replica_terms",
		"SELECT VERSION()",
		"mysql_major",
		"mysql_minor",
		"SHOW SLAVE STATUS",
		"SHOW REPLICA STATUS",
		"SHOW SLAVE HOSTS",
		"SHOW REPLICAS",
		"show_slave_status",
		"show_replica_status",
		"${replica_status_cmd}",
		"${replica_hosts_cmd}",
	}
	for _, s := range required {
		if !strings.Contains(script, s) {
			t.Fatalf("expected script to contain %q, got: %s", s, script)
		}
	}
}

func TestBuildBashPreStop_versionGateMatchesAtLeastMySQL84(t *testing.T) {
	versionFn := preStopVersionFnScript(t, buildBashPreStop())
	cases := []struct {
		version string
		exp     bool
	}{
		{"8.0.34", false},
		{"8.0.34-26", false},
		{"8.3.9", false},
		{"8.4.0", true},
		{"8.4.0-28", true},
		{"8.4.1", true},
		{"9.0.0", true},
		{"9.10.0", true},
		{"10.5.3", true},
	}
	for _, tc := range cases {
		goWant := mysqlversion.AtLeastMySQL84(semver.MustParse(strings.SplitN(tc.version, "-", 2)[0]))
		got, err := runPreStopVersionFn(versionFn, tc.version)
		if err != nil {
			t.Fatalf("%q: %v", tc.version, err)
		}
		if got != goWant || got != tc.exp {
			t.Fatalf("%q: gate=%v AtLeastMySQL84=%v want %v", tc.version, got, goWant, tc.exp)
		}
	}
}

func TestBuildBashPreStop_versionGateUsesEnvOrServerVersion(t *testing.T) {
	gate := preStopVersionGateScript(t, buildBashPreStop())
	cases := []struct {
		name     string
		env      string
		detected string
		exp      bool
	}{
		{"env 8.4", "8.4.0", "", true},
		{"env 8.0", "8.0.34", "", false},
		{"detected 8.4 when env empty", "", "8.4.0-28", true},
		{"detected 8.0 when env empty", "", "8.0.34-26", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runPreStopVersionGate(t, gate, tc.env, tc.detected)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Fatalf("got %v want %v", got, tc.exp)
			}
		})
	}
}

func preStopVersionFnScript(t *testing.T, script string) string {
	t.Helper()
	start := strings.Index(script, "prestop_mysql_use_source_replica_terms()")
	end := strings.Index(script, "use_source_replica_terms=0")
	if start < 0 || end <= start {
		t.Fatal("could not extract version function from preStop script")
	}
	return script[start:end]
}

func preStopVersionGateScript(t *testing.T, script string) string {
	t.Helper()
	start := strings.Index(script, "prestop_mysql_use_source_replica_terms()")
	end := strings.Index(script, `if [ "${use_source_replica_terms}" -eq 1 ]; then`)
	if start < 0 || end <= start {
		t.Fatal("could not extract version gate from preStop script")
	}
	return script[start:end]
}

func runPreStopVersionFn(versionFn, version string) (bool, error) {
	cmd := exec.Command("bash", "-c", versionFn+`prestop_mysql_use_source_replica_terms "$1"`, "prestop-test", version)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func runPreStopVersionGate(t *testing.T, gate, version, detected string) (bool, error) {
	t.Helper()
	binDir := t.TempDir()
	mysqlStub := filepath.Join(binDir, "mysql")
	if err := os.WriteFile(mysqlStub, []byte(`#!/bin/bash
for arg in "$@"; do
  if [ "$arg" = "SELECT VERSION()" ]; then
    echo "${DETECTED_MYSQL_VERSION}"
    exit 0
  fi
done
exit 1
`), 0755); err != nil {
		return false, err
	}

	script := gate + `
if [ "${use_source_replica_terms}" -eq 1 ]; then exit 0; else exit 1; fi`
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = []string{
		"MY_MYSQL_VERSION=" + version,
		"DETECTED_MYSQL_VERSION=" + detected,
		"PATH=" + binDir,
	}
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func TestBuildBashPreStop_preservesOrchestratorLogic(t *testing.T) {
	script := buildBashPreStop()

	required := []string{
		"graceful-master-takeover-auto",
		"MY_FQDN",
		"read_only_status",
		"ORCH_HTTP_API",
		"ORCH_CLUSTER_ALIAS",
	}
	for _, s := range required {
		if !strings.Contains(script, s) {
			t.Fatalf("expected script to contain %q, got: %s", s, script)
		}
	}
}
