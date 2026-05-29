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
package mysqlversioning

import "testing"

func TestSourceReplicaGrantHints_usesSlaveMasterInfoCatalog(t *testing.T) {
	t.Parallel()
	h := sourceReplicaGrantHints()
	if h.OrchestratorMetadataTable != "mysql.slave_master_info" {
		t.Fatalf("OrchestratorMetadataTable: want mysql.slave_master_info, got %q", h.OrchestratorMetadataTable)
	}
}

func TestMasterSlaveGrantHintsMySQL8_matchesSourceReplicaOrchestratorCatalog(t *testing.T) {
	t.Parallel()
	masterSlave := masterSlaveGrantHintsMySQL8()
	sourceReplica := sourceReplicaGrantHints()
	if masterSlave.OrchestratorMetadataTable != sourceReplica.OrchestratorMetadataTable {
		t.Fatalf("master/slave %q vs source/replica %q", masterSlave.OrchestratorMetadataTable, sourceReplica.OrchestratorMetadataTable)
	}
}
