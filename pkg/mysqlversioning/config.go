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

import (
	"github.com/codecapsules-io/mysql-operator/pkg/util/mysqlversion"
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

// Mysqld5xConfigs returns mysqld key/value defaults for MySQL 5.x lines.
func Mysqld5xConfigs() map[string]string {
	return map[string]string{
		"query-cache-type": "0",
		"query-cache-size": "0",
		"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER," +
			"NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
		"expire-logs-days": "14",
	}
}

// Mysqld8xConfigs returns mysqld key/value defaults for MySQL 8.x lines.
func Mysqld8xConfigs() map[string]string {
	return map[string]string{
		"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION," +
			"NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",
		"binlog_expire_logs_seconds": "1209600", // 14 days = 14 * 24 * 60 * 60
	}
}

// Mysqld80AuthPluginConfig is not applied on MySQL 8.4+ where mysql_native_password is unavailable.
func Mysqld80AuthPluginConfig() map[string]string {
	return map[string]string{
		"default-authentication-plugin": "mysql_native_password",
	}
}

// MysqldBooleanConfigs returns mysqld boolean options for the given server version.
func MysqldBooleanConfigs(v semver.Version) []string {
	out := []string{
		"skip-name-resolve",
	}
	if !mysqlversion.AtLeastMySQL8030(v) {
		out = append(out, "skip-host-cache")
	}
	return out
}
