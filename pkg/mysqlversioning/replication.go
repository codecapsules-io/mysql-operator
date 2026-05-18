/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

// ReplicationDialect holds replication control and introspection SQL for a server line.
type ReplicationDialect struct {
	StopReplication string
	// ChangeSourceSQL uses placeholders: MASTER_HOST/SOURCE_HOST order in exec args.
	ChangeSourceSQL  string
	StartReplication string
	FallbackStartSQL string
	// ResetBinaryLogsForGTID is used inside SetPurgedGTID transaction; empty uses ResetMaster.
	ResetBinaryLogsForGTID string
	ResetMaster            string
	ShowReplicaStatusCmd   string
	ShowReplicasCmd        string
	LogLabelPreStop        string
}

// MasterSlaveReplication is for MySQL / Percona Server before 8.4 replication terminology.
func MasterSlaveReplication() ReplicationDialect {
	return ReplicationDialect{
		StopReplication: "STOP SLAVE;",
		ChangeSourceSQL: `
	  CHANGE MASTER TO MASTER_AUTO_POSITION=1,
		MASTER_HOST=?,
		MASTER_USER=?,
		MASTER_PASSWORD=?,
		MASTER_CONNECT_RETRY=?;
	`,
		StartReplication: "START SLAVE;",
		FallbackStartSQL: `
		  reset slave;
		  start slave IO_THREAD;
		  stop slave IO_THREAD;
		  reset slave;
		  start slave;
		`,
		ResetMaster:            "RESET MASTER;",
		ResetBinaryLogsForGTID: "",
		ShowReplicaStatusCmd:   `SHOW SLAVE STATUS\G`,
		ShowReplicasCmd:        `SHOW SLAVE HOSTS\G`,
		LogLabelPreStop:        "show_slave_status",
	}
}

// SourceReplicaReplication is for MySQL 8.4+ / 9.x replication terminology.
func SourceReplicaReplication() ReplicationDialect {
	return ReplicationDialect{
		StopReplication: "STOP REPLICA;",
		ChangeSourceSQL: `
	  CHANGE REPLICATION SOURCE TO SOURCE_AUTO_POSITION=1,
		SOURCE_HOST=?,
		SOURCE_USER=?,
		SOURCE_PASSWORD=?,
		SOURCE_CONNECT_RETRY=?;
	`,
		StartReplication: "START REPLICA;",
		FallbackStartSQL: `
		  RESET REPLICA;
		  START REPLICA IO_THREAD;
		  STOP REPLICA IO_THREAD;
		  RESET REPLICA;
		  START REPLICA;
		`,
		ResetMaster:            "",
		ResetBinaryLogsForGTID: "RESET BINARY LOGS AND GTIDS;",
		ShowReplicaStatusCmd:   `SHOW REPLICA STATUS\G`,
		ShowReplicasCmd:        `SHOW REPLICAS\G`,
		LogLabelPreStop:        "show_replica_status",
	}
}

// ResetBinaryLogsStatement returns the statement to clear binary logs before SET GTID_PURGED.
func (d ReplicationDialect) ResetBinaryLogsStatement() string {
	if d.ResetBinaryLogsForGTID != "" {
		return d.ResetBinaryLogsForGTID
	}
	return d.ResetMaster
}
