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
package mysqlcluster_test

import (
	"testing"

	"github.com/blang/semver"

	syncer "github.com/bitpoke/mysql-operator/pkg/controller/mysqlcluster/internal/syncer"
)

func TestMysqlKVConfigsForVersion_legacy80(t *testing.T) {
	m := syncer.MysqlKVConfigsForVersion(semver.MustParse("8.0.20"))
	if _, ok := m["skip-slave-start"]; !ok {
		t.Fatalf("expected skip-slave-start for 8.0.20")
	}
	if _, ok := m["innodb-log-files-in-group"]; !ok {
		t.Fatalf("expected innodb-log-files-in-group for 8.0.20")
	}
	if _, ok := m["skip-replica-start"]; ok {
		t.Fatalf("did not expect skip-replica-start for 8.0.20")
	}
}

func TestMysqlKVConfigsForVersion_sourceReplica84(t *testing.T) {
	m := syncer.MysqlKVConfigsForVersion(semver.MustParse("8.4.0"))
	if _, ok := m["skip-replica-start"]; !ok {
		t.Fatalf("expected skip-replica-start for 8.4")
	}
	if _, ok := m["skip-slave-start"]; ok {
		t.Fatalf("did not expect skip-slave-start for 8.4")
	}
	if _, ok := m["innodb-log-files-in-group"]; ok {
		t.Fatalf("did not expect innodb-log-files-in-group for 8.4")
	}
}

