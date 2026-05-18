/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
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
