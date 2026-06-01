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
package sidecar

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

const defaultAuthMigrateTargetPlugin = "caching_sha2_password"

// RunAuthMigrate migrates persistent mysql_native_password accounts to targetPlugin using
// root over the local Unix socket (required for root@localhost and SYSTEM_USER).
func RunAuthMigrate(cfg *Config, targetPlugin string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if strings.TrimSpace(cfg.RootPassword) == "" {
		return fmt.Errorf("MYSQL_ROOT_PASSWORD is required for auth plugin migration")
	}
	targetPlugin = strings.TrimSpace(targetPlugin)
	if targetPlugin == "" {
		targetPlugin = defaultAuthMigrateTargetPlugin
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	db, err := openRootSocketDB(cfg.RootPassword)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if pre84Err := assertServerPre84(ctx, db); pre84Err != nil {
		return pre84Err
	}
	if writePrimaryErr := assertWritablePrimary(ctx, db); writePrimaryErr != nil {
		return writePrimaryErr
	}

	appUser := strings.TrimSpace(cfg.AppUser)
	appPass := cfg.AppPassword
	opUser := strings.TrimSpace(cfg.OperatorUser)
	orcUser := strings.TrimSpace(cfg.OrchestratorUser)

	accounts, err := listNativePasswordAccounts(ctx, db, opUser, orcUser)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		log.Info("no persistent mysql_native_password accounts to migrate")
		return nil
	}

	alters, err := buildAlterStatements(ctx, db, accounts, targetPlugin, cfg.RootPassword, appUser, appPass)
	if err != nil {
		return err
	}

	log.Info("applying auth plugin migration via socket as root", "statements", len(alters))
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	for _, stmt := range alters {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute %q: %w", stmt, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	log.Info("auth plugin migration complete", "accounts", len(accounts), "targetPlugin", targetPlugin)
	return nil
}

type mysqlAccount struct {
	user string
	host string
}

func openRootSocketDB(rootPassword string) (*sql.DB, error) {
	sock := fmt.Sprintf("%s/mysql.sock", constants.DataVolumeMountPath)
	dsn := fmt.Sprintf("%s:%s@unix(%s)/?timeout=30s&multiStatements=false",
		url.QueryEscape("root"), url.QueryEscape(rootPassword), sock)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql socket connection: %w", err)
	}
	db.SetConnMaxLifetime(time.Minute)
	db.SetMaxOpenConns(2)
	return db, nil
}

func assertServerPre84(ctx context.Context, db *sql.DB) error {
	var ver string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&ver); err != nil {
		return fmt.Errorf("read server version: %w", err)
	}
	log.Info("auth migrate server version", "version", ver)
	parsed, err := semver.Make(strings.SplitN(ver, "-", 2)[0])
	if err != nil {
		return fmt.Errorf("parse server version %q: %w", ver, err)
	}
	if parsed.Major == 8 && parsed.Minor >= 4 || parsed.Major >= 9 {
		return fmt.Errorf("server is already on %s; auth migration must complete before the 8.4 image rollout", ver)
	}
	return nil
}

func assertWritablePrimary(ctx context.Context, db *sql.DB) error {
	for i := 0; i < 12; i++ {
		var readOnly int
		err := db.QueryRowContext(ctx, "SELECT @@GLOBAL.read_only OR @@GLOBAL.super_read_only").Scan(&readOnly)
		if err == nil && readOnly == 0 {
			return nil
		}
		log.Info("waiting for writable primary", "attempt", i+1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("primary is read-only; run auth migration on the writable master")
}

func listNativePasswordAccounts(ctx context.Context, db *sql.DB, operatorUser, orchestratorUser string) ([]mysqlAccount, error) {
	exclude := []string{"mysql.infoschema", "mysql.session", "mysql.sys", operatorUser, "sys_replication", "sys_exporter", "sys_heartbeat"}
	if orchestratorUser != "" {
		exclude = append(exclude, orchestratorUser)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(exclude)), ",")
	query := fmt.Sprintf(`
		SELECT user, host
		FROM mysql.user
		WHERE plugin = 'mysql_native_password'
		  AND user NOT IN (%s)
		ORDER BY (user = 'root') DESC, user, host`, placeholders)

	args := make([]interface{}, len(exclude))
	for i, u := range exclude {
		args[i] = u
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list accounts to migrate: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var accounts []mysqlAccount
	for rows.Next() {
		var acct mysqlAccount
		if err := rows.Scan(&acct.user, &acct.host); err != nil {
			return nil, err
		}
		accounts = append(accounts, acct)
	}
	return accounts, rows.Err()
}

func buildAlterStatements(ctx context.Context, db *sql.DB, accounts []mysqlAccount, targetPlugin, rootPass, appUser, appPass string) ([]string, error) {
	stmts := make([]string, 0, len(accounts))
	for _, acct := range accounts {
		qu, err := quoteLiteral(ctx, db, acct.user)
		if err != nil {
			return nil, err
		}
		qh, err := quoteLiteral(ctx, db, acct.host)
		if err != nil {
			return nil, err
		}
		pass := passwordForUser(acct.user, rootPass, appUser, appPass)
		if pass != "" {
			pq, err := quoteLiteral(ctx, db, pass)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, fmt.Sprintf("ALTER USER %s@%s IDENTIFIED WITH %s BY %s", qu, qh, targetPlugin, pq))
		} else {
			stmts = append(stmts, fmt.Sprintf("ALTER USER %s@%s IDENTIFIED WITH %s RETAIN CURRENT PASSWORD", qu, qh, targetPlugin))
		}
	}
	return stmts, nil
}

func passwordForUser(user, rootPass, appUser, appPass string) string {
	switch user {
	case "root":
		return rootPass
	default:
		if appUser != "" && user == appUser {
			return appPass
		}
	}
	return ""
}

func quoteLiteral(ctx context.Context, db *sql.DB, value string) (string, error) {
	var quoted string
	if err := db.QueryRowContext(ctx, "SELECT QUOTE(?)", value).Scan(&quoted); err != nil {
		return "", fmt.Errorf("quote literal: %w", err)
	}
	return quoted, nil
}
