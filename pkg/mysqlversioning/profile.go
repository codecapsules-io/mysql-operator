/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"fmt"

	"github.com/blang/semver"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/util/mysqlversion"
)

// Profile describes operator-owned behavior for a Percona Server / MySQL line.
type Profile interface {
	Name() string
	Matches(v semver.Version) bool
	// MySQLOperatorKV returns operator defaults merged under user mysqlConf (server version for fine gates).
	MySQLOperatorKV(v semver.Version) map[string]string
	UseMySQL5xConfigs() bool
	UseMySQL8xConfigs() bool
	UseMySQL80AuthPlugin() bool
	Replication() ReplicationDialect
	GrantHints() GrantHints
	SidecarProfileKey() string
	WantsPerconaInitContainer(v semver.Version) bool
	// PodSecurityHints returns pod security defaults for this version line. perconaServerImage is
	// true when the resolved server image is Percona (e.g. docker.io/percona/...); official PS 8.4+
	// uses UID/GID 1001 for mysqld while operator sidecars stay on 999 at pod level when pod runAsUser is omitted.
	PodSecurityHints(perconaServerImage bool) PodSecurityHints
	Validate(spec *api.MysqlClusterSpec) error
	// InnoDBOperatorLogSizing returns the mysqlConf option key and size in bytes for InnoDB log defaults
	// derived from computeInnodbLogFileSize (per-file). Legacy servers use innodb-log-file-size = perFile;
	// MySQL 8.0.30+ uses innodb-redo-log-capacity = 2×perFile (matching former innodb_log_files_in_group=2).
	InnoDBOperatorLogSizing(v semver.Version, perFileBytes int64) (optionKey string, sizeBytes int64)
}

// OperatorKVCommon returns shared mysqld defaults (replication naming varies by terminology flag).
func OperatorKVCommon(v semver.Version, useSourceReplicaTerminology bool) map[string]string {
	out := map[string]string{
		"log-bin":                        "/var/lib/mysql/mysql-bin",
		"relay-log-recovery":             "on",
		"default-storage-engine":         "InnoDB",
		"gtid-mode":                      "on",
		"enforce-gtid-consistency":       "on",
		"key-buffer-size":                "32M",
		"myisam-recover-options":         "FORCE,BACKUP",
		"max-allowed-packet":             "16M",
		"max-connect-errors":             "1000000",
		"sysdate-is-now":                 "1",
		"sync-binlog":                    "1",
		"binlog-format":                  "ROW",
		"tmp-table-size":                 "32M",
		"max-heap-table-size":            "32M",
		"max-connections":                "500",
		"thread-cache-size":              "50",
		"open-files-limit":               "65535",
		"table-definition-cache":         "4096",
		"table-open-cache":               "4096",
		"innodb-flush-method":            "O_DIRECT",
		"innodb-flush-log-at-trx-commit": "2",
		"innodb-file-per-table":          "1",
		"character-set-server":           "utf8mb4",
		"collation-server":               "utf8mb4_unicode_ci",
	}
	if !mysqlversion.AtLeastMySQL8030(v) {
		out["innodb-log-files-in-group"] = "2"
	}
	if useSourceReplicaTerminology {
		out["log-replica-updates"] = "on"
		out["skip-replica-start"] = "on"
	} else {
		out["log-slave-updates"] = "on"
		out["skip-slave-start"] = "on"
		out["relay-log-info-repository"] = "TABLE"
		out["master-info-repository"] = "TABLE"
	}
	return out
}

// DefaultValidate is a conservative validator shared by built-in profiles.
func DefaultValidate(spec *api.MysqlClusterSpec) error {
	if spec.SecretName == "" {
		return fmt.Errorf("secretName is required")
	}
	return nil
}

// OperatorKVForVersion returns merged operator mysqld defaults for v, using the process registry when initialized.
func OperatorKVForVersion(v semver.Version) map[string]string {
	return ProfileFor(v).MySQLOperatorKV(v)
}
