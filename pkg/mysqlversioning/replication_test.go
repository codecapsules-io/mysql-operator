/*
Copyright 2026 Pressinfra SRL

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

package mysqlversioning

import (
	"strings"
	"testing"
)

func TestSourceReplicaReplication_usesGetSourcePublicKey(t *testing.T) {
	sql := SourceReplicaReplication().ChangeSourceSQL
	if !strings.Contains(sql, "GET_SOURCE_PUBLIC_KEY=1") {
		t.Fatalf("expected GET_SOURCE_PUBLIC_KEY=1 in ChangeSourceSQL, got:\n%s", sql)
	}
}

func TestMasterSlaveReplication_doesNotUseGetSourcePublicKey(t *testing.T) {
	sql := MasterSlaveReplication().ChangeSourceSQL
	if strings.Contains(sql, "GET_SOURCE_PUBLIC_KEY") || strings.Contains(sql, "GET_MASTER_PUBLIC_KEY") {
		t.Fatalf("5.7/8.0 dialect must not set public key options, got:\n%s", sql)
	}
}
