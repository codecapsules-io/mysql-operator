/*
Copyright 2018 Pressinfra SRL

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

package sidecar

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-ini/ini"

	"github.com/bitpoke/mysql-operator/pkg/mysqlversioning"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

// RunConfigCommand generates my.cnf, client.cnf and 10-dynamic.cnf files.
// nolint: gocyclo
func RunConfigCommand(cfg *Config) error {
	prof := mysqlversioning.ProfileFor(cfg.MySQLVersion)
	gh := prof.GrantHints()
	log.Info("RunConfigCommand start",
		"host", cfg.Hostname,
		"cluster", cfg.ClusterName,
		"namespace", cfg.Namespace,
		"mysqlVersion", cfg.MySQLVersion.String(),
		"profile", prof.Name(),
		"existsMySQLData", cfg.ExistsMySQLData,
		"orchestratorMetadataTable", gh.OrchestratorMetadataTable,
		"heartbeatUser", cfg.HeartBeatUser,
		"heartbeatPasswordLen", len(cfg.HeartBeatPassword),
		"operatorUser", cfg.OperatorUser,
		"operatorPasswordLen", len(cfg.OperatorPassword),
	)
	var err error

	if err = copyFile(mountConfigDir+"/my.cnf", configDir+"/my.cnf"); err != nil {
		return fmt.Errorf("copy file my.cnf: %s", err)
	}

	if err = copyFile(mountConfigDir+"/"+shPreStop, configDir+"/"+shPreStop); err != nil {
		return fmt.Errorf("copy file %s: %s", shPreStop, err)
	}

	if err = os.Mkdir(confDPath, os.FileMode(0755)); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error mkdir %s/conf.d: %s", configDir, err)
		}
	}

	reportHost := cfg.FQDNForServer(cfg.ServerID())

	var identityCFG, initCFG, clientCFG, heartbeatCFG *ini.File

	// mysql server identity configs
	if identityCFG, err = getIdentityConfigs(cfg.ServerID(), reportHost); err != nil {
		return fmt.Errorf("failed to get dynamic configs: %s", err)
	}
	if err = identityCFG.SaveTo(path.Join(confDPath, "10-identity.cnf")); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	// write initialization sql file. This file is the init-file used by MySQL to configure itself
	var gtidPurged string
	gtidPurged, err = readPurgedGTID()
	if err != nil {
		// not a fatal error, log it and continue
		log.Info("error while reading PURGE GTID from xtrabackup_binlog_info", "error", err)
	}

	initFilePath := path.Join(confDPath, "operator-init.sql")
	initBody := initFileQuery(cfg, gtidPurged)
	if err = ioutil.WriteFile(initFilePath, initBody, 0644); err != nil {
		return fmt.Errorf("failed to write init-file: %s", err)
	}
	approxStmts := strings.Count(string(initBody), ";\n") + 1
	log.Info("wrote operator-init.sql",
		"path", initFilePath,
		"bytes", len(initBody),
		"approxStatements", approxStmts,
		"gtidPurgedLen", len(gtidPurged),
	)

	// mysql server utility user configs
	if initCFG, err = getInitFileConfigs(initFilePath); err != nil {
		return fmt.Errorf("failed to configure init file: %s", err)
	}
	if err = initCFG.SaveTo(path.Join(confDPath, "10-init-file.cnf")); err != nil {
		return fmt.Errorf("failed to configure init file: %s", err)
	}

	// mysql client connect credentials
	if clientCFG, err = getClientConfigs(cfg.OperatorUser, cfg.OperatorPassword); err != nil {
		return fmt.Errorf("failed to get client configs: %s", err)
	}

	if err = clientCFG.SaveTo(confClientPath); err != nil {
		return fmt.Errorf("failed to save configs: %s", err)
	}

	// mysql heartbeat: Unix socket avoids Perl DBD::mysql + caching_sha2_password TCP/RSA quirks
	// ("Authentication requires secure connection" despite get-server-public-key in option files).
	if heartbeatCFG, err = getHeartbeatClientConfigs(cfg.HeartBeatUser, cfg.HeartBeatPassword); err != nil {
		return fmt.Errorf("failed to get heartbeat configs: %s", err)
	}

	if err = heartbeatCFG.SaveTo(confHeartbeatPath); err != nil {
		return fmt.Errorf("failed to save heartbeat configs: %s", err)
	}

	if err = writeLoopbackClientHints(); err != nil {
		return fmt.Errorf("failed to write loopback client hints: %s", err)
	}

	log.Info("RunConfigCommand completed",
		"initFile", initFilePath,
		"clientConf", confClientPath,
		"heartbeatConf", confHeartbeatPath,
		"loopbackHints", constants.ConfClientLoopbackPath,
		"heartbeatSocket", path.Join(constants.DataVolumeMountPath, "mysql.sock"),
	)
	return nil
}

// writeLoopbackClientHints writes a defaults file without credentials so operators can run, e.g.:
// mysql --defaults-file=/etc/mysql/client-loopback.cnf -uroot -p
// over 127.0.0.1 with caching_sha2_password without TLS (RSA public key exchange).
func writeLoopbackClientHints() error {
	cfg := ini.Empty()
	client := cfg.Section("client")
	if _, err := client.NewKey("host", "127.0.0.1"); err != nil {
		return err
	}
	if _, err := client.NewKey("port", mysqlPort); err != nil {
		return err
	}
	if _, err := client.NewKey("get-server-public-key", "1"); err != nil {
		return err
	}
	return cfg.SaveTo(constants.ConfClientLoopbackPath)
}

func getClientConfigs(user, pass string) (*ini.File, error) {
	cfg := ini.Empty()
	// create client.cnf file
	client := cfg.Section("client")

	if _, err := client.NewKey("host", "127.0.0.1"); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("port", mysqlPort); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("user", user); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("password", pass); err != nil {
		return nil, err
	}
	// caching_sha2_password (MySQL 8+ default) over loopback TCP without TLS needs the server RSA
	// public key exchange; otherwise clients fail with "Authentication requires secure connection".
	// Safe for 127.0.0.1-only utility accounts (operator client, metrics exporter). See:
	// https://dev.mysql.com/doc/refman/8.0/en/caching-sha2-pluggable-authentication.html
	if _, err := client.NewKey("get-server-public-key", "1"); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getHeartbeatClientConfigs builds [client] for pt-heartbeat: Unix socket to local mysqld only
// (same pod; datadir mounted read-only on the heartbeat container). No TCP — avoids Perl DBD::mysql
// ignoring get-server-public-key from defaults with caching_sha2_password.
func getHeartbeatClientConfigs(user, pass string) (*ini.File, error) {
	cfg := ini.Empty()
	client := cfg.Section("client")

	sock := path.Join(constants.DataVolumeMountPath, "mysql.sock")
	if _, err := client.NewKey("socket", sock); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("user", user); err != nil {
		return nil, err
	}
	if _, err := client.NewKey("password", pass); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getIdentityConfigs(id int, reportHost string) (*ini.File, error) {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	if _, err := mysqld.NewKey("server-id", strconv.Itoa(id)); err != nil {
		return nil, err
	}
	if _, err := mysqld.NewKey("report-host", reportHost); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getInitFileConfigs(filePath string) (*ini.File, error) {
	cfg := ini.Empty()
	mysqld := cfg.Section("mysqld")

	if _, err := mysqld.NewKey("init-file", filePath); err != nil {
		return nil, err
	}

	return cfg, nil
}

func initFileQuery(cfg *Config, gtidPurged string) []byte {
	queries := []string{
		"SET @@SESSION.SQL_LOG_BIN = 0",
	}

	hints := mysqlversioning.ProfileFor(cfg.MySQLVersion).GrantHints()

	// Create sys_operator and status before READ_ONLY and user DDL. If a later statement
	// fails on a newer server (grant syntax, etc.), init-file would otherwise stop before
	// the status table exists and probes / node init see Error 1146.
	queries = append(queries, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", toolsDbName))

	// when the status.ibd file does not exist, need to delete the status table
	_, err := os.Stat(path.Join(dataDir, constants.OperatorDbName, constants.OperatorStatusTableName+".ibd"))
	if os.IsNotExist(err) {
		queries = append(queries, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s",
			constants.OperatorDbName, constants.OperatorStatusTableName))
	}

	// nolint: gosec
	queries = append(queries, fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %[1]s.%[2]s ("+
			"  name varchar(64) PRIMARY KEY,"+
			"  value varchar(8192) NOT NULL\n)",
		constants.OperatorDbName, constants.OperatorStatusTableName))

	// nolint: gosec
	queries = append(queries, fmt.Sprintf("REPLACE INTO %s.%s VALUES ('%s', '0')",
		constants.OperatorDbName, constants.OperatorStatusTableName, "configured"))

	// set server as read only
	// https://github.com/bitpoke/mysql-operator/issues/509
	queries = append(queries, "SET GLOBAL READ_ONLY = 1")

	// configure operator utility user
	queries = append(queries, createUserQuery(cfg.OperatorUser, cfg.OperatorPassword, "%",
		[]string{"SUPER", "SHOW DATABASES", "PROCESS", "RELOAD", "CREATE", "CREATE USER", "SELECT"}, "*.*",
		[]string{"REPLICATION SLAVE"}, "*.*",
		[]string{"ALL"}, fmt.Sprintf("%s.*", toolsDbName))...)

	replMetaTable := hints.OrchestratorMetadataTable

	// configure orchestrator user
	queries = append(queries, createUserQuery(cfg.OrchestratorUser, cfg.OrchestratorPassword, "%",
		[]string{"SUPER", "PROCESS", "REPLICATION SLAVE", "REPLICATION CLIENT", "RELOAD"}, "*.*",
		[]string{"SELECT"}, replMetaTable,
		[]string{"SELECT", "CREATE"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName))...)

	// configure replication user
	replPermissions := hints.ReplicationUserPrivileges
	queries = append(queries, createUserQuery(cfg.ReplicationUser, cfg.ReplicationPassword, "%",
		replPermissions, "*.*")...)

	// configure metrics exporter user
	queries = append(queries, createUserQuery(cfg.MetricsUser, cfg.MetricsPassword, "127.0.0.1",
		[]string{"SELECT", "PROCESS", "REPLICATION CLIENT"}, "*.*",
		[]string{"SELECT", "CREATE"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName))...)

	queries = append(queries, fmt.Sprintf("ALTER USER %s@'127.0.0.1' WITH MAX_USER_CONNECTIONS 3", cfg.MetricsUser))

	// configure heartbeat user (127.0.0.1 for TCP tools; localhost for Unix socket / pt-heartbeat)
	// because of pt-heartbeat make sure not to have ALL or SUPER privileges:
	// https://github.com/percona/percona-toolkit/blob/e85ce15ef24bc4614b4d2f13792fa73583d68f8e/bin/pt-heartbeat#L6433
	for _, host := range []string{"127.0.0.1", "localhost"} {
		queries = append(queries, createUserQuery(cfg.HeartBeatUser, cfg.HeartBeatPassword, host,
			[]string{"CREATE", "SELECT", "DELETE", "UPDATE", "INSERT"}, fmt.Sprintf("%s.%s", toolsDbName, toolsHeartbeatTableName),
			[]string{"REPLICATION CLIENT"}, "*.*")...)
	}

	if len(gtidPurged) != 0 {
		// if gtid is found in the backup then set it in the status table to be processed by the operator
		// nolint: gosec
		queries = append(queries, fmt.Sprintf(`REPLACE INTO %s.%s VALUES ('%s', '%s')`,
			constants.OperatorDbName, constants.OperatorStatusTableName, "backup_gtid_purged", gtidPurged))
	}

	// if just recently the node was initialized from a backup then replication must be reset
	// to avoid replicating from a previous source.
	if cfg.ShouldCloneFromBucket() {
		queries = append(queries, hints.ResetReplicationAll)
	}

	for _, stmt := range cfg.InitFileExtraSQL {
		if strings.TrimSpace(stmt) != "" {
			queries = append(queries, stmt)
		}
	}

	return []byte(strings.Join(queries, ";\n") + ";\n")
}

func createUserQuery(name, pass, host string, rights ...interface{}) []string {
	user := fmt.Sprintf("%s@'%s'", name, host)

	queries := []string{
		fmt.Sprintf("DROP USER IF EXISTS %s", user),
		fmt.Sprintf("CREATE USER %s IDENTIFIED BY '%s'", user, pass),
	}

	if len(rights)%2 != 0 {
		panic("not a good number of parameters")
	}
	grants := []string{}
	for i := 0; i < len(rights); i += 2 {
		var (
			right []string
			on    string
			ok    bool
		)
		if right, ok = rights[i].([]string); !ok {
			panic("[right] not a good parameter")
		}
		if on, ok = rights[i+1].(string); !ok {
			panic("[on] not a good parameter")
		}
		grant := fmt.Sprintf("GRANT %s ON %s TO %s", strings.Join(right, ", "), on, user)
		grants = append(grants, grant)
	}

	return append(queries, grants...)
}
