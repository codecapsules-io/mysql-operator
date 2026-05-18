/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

// GrantHints drives init-file grants for operator-managed users.
type GrantHints struct {
	OrchestratorMetadataTable string
	ReplicationUserPrivileges []string
	ResetReplicationAll       string
}

func masterSlaveGrantHintsMySQL57() GrantHints {
	return GrantHints{
		OrchestratorMetadataTable: "mysql.slave_master_info",
		ReplicationUserPrivileges: []string{
			"SELECT", "PROCESS", "RELOAD", "LOCK TABLES", "REPLICATION CLIENT", "REPLICATION SLAVE",
		},
		ResetReplicationAll: "RESET SLAVE ALL",
	}
}

func masterSlaveGrantHintsMySQL8() GrantHints {
	h := masterSlaveGrantHintsMySQL57()
	h.ReplicationUserPrivileges = append(h.ReplicationUserPrivileges, "BACKUP_ADMIN")
	return h
}

func sourceReplicaGrantHints() GrantHints {
	h := masterSlaveGrantHintsMySQL8()
	// Orchestrator needs SELECT on the server’s replication connection metadata table.
	// Through MySQL 8.4 / Percona 8.4 the physical InnoDB table remains mysql.slave_master_info
	// (SQL and status commands use “replica” wording; there is no mysql.replica_master_info table).
	// GRANT ON mysql.replica_master_info fails with ER_NO_SUCH_TABLE and aborts init-file before
	// later users (e.g. sys_heartbeat) are created — fix the catalog name, do not paper over with DDL.
	h.OrchestratorMetadataTable = "mysql.slave_master_info"
	h.ResetReplicationAll = "RESET REPLICA ALL"
	return h
}
