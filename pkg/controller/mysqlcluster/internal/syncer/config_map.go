/*
Copyright 2018 Pressinfra SRL
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

package mysqlcluster

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/go-ini/ini"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/presslabs/controller-util/syncer"

	"github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/versionupgrade"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/mysqlversioning"
	"github.com/codecapsules-io/mysql-operator/pkg/util/mysqlversion"
)

// NewConfigMapSyncer returns config map syncer.
// sts may be nil; when status.appliedMysqlVersion is unset, a lagging StatefulSet template is used for preStop SQL.
func NewConfigMapSyncer(c client.Client, scheme *runtime.Scheme, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) syncer.Interface {
	cm := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.GetNameForResource(mysqlcluster.ConfigMap),
			Namespace: cluster.Namespace,
		},
	}

	return syncer.NewObjectSyncer("ConfigMap", cluster.Unwrap(), cm, c, func() error {
		cm.ObjectMeta.Labels = cluster.GetLabels()
		cm.ObjectMeta.Labels["generated"] = "true"

		data, err := buildMysqlConfData(c, cluster, sts)
		if err != nil {
			return fmt.Errorf("failed to create mysql configs: %s", err)
		}

		cm.Data = map[string]string{
			"my.cnf": data,
		}

		if cluster.Spec.PodSpec.MysqlLifecycle == nil {
			cm.Data[shPreStopFile] = buildBashPreStop(cluster, sts)
		}

		return nil
	})
}

func buildBashPreStop(cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) string {
	// preStop runs against the mysqld process on this pod; applied status lags during rollout.
	v := cluster.EffectiveVersion(sts)
	d := mysqlversioning.ProfileFor(v).Replication()
	replicaStatusCmd := d.ShowReplicaStatusCmd
	replicaHostsCmd := d.ShowReplicasCmd
	logLabel := d.LogLabelPreStop
	data := `#!/bin/bash
set -ex

current=$(date "+%Y-%m-%d %H:%M:%S")
echo "[${current}]preStop is ongoing"
read_only_status=$(mysql --defaults-file=ConfClientPathHolder -NB -e 'SELECT @@read_only')
replica_status=$(mysql --defaults-file=ConfClientPathHolder -NB -e '__REPLICA_STATUS_CMD__')
# orchestrator will isolate old master during failover
has_replica_hosts=$(mysql --defaults-file=ConfClientPathHolder -NB -e '__REPLICA_HOSTS_CMD__')
replica_status_count=$(echo -n "$replica_status" | wc -l )
has_replica_count=$(echo -n "$has_replica_hosts" | wc -l )
echo "hostname=$(hostname) readonly=${read_only_status} __LOG_LABEL__=${replica_status_count}"
echo "has_replica_hosts=${has_replica_count}"
if [ ${read_only_status} -eq 0  ] && [ ${replica_status_count} -eq 0 ] && [ ${has_replica_count} -gt 0 ]
then
		masterhostname=$( curl  -s "${ORCH_HTTP_API}/master/${ORCH_CLUSTER_ALIAS}" |  awk -F":" '{print $3}' | awk -F'"' '{print $2}' )
        echo "master from orchestrator: ${masterhostname}"
        if [ "${MY_FQDN}" == "${masterhostname}" ]
        then
                curl  -s "${ORCH_HTTP_API}/graceful-master-takeover-auto/${ORCH_CLUSTER_ALIAS}"
				echo "graceful-master-takeover-auto is ongoing, sleep 5 seconds in order to make sure service can work well."
				sleep 5
        fi
fi
`
	data = strings.Replace(data, "__REPLICA_STATUS_CMD__", replicaStatusCmd, 1)
	data = strings.Replace(data, "__REPLICA_HOSTS_CMD__", replicaHostsCmd, 1)
	data = strings.Replace(data, "__LOG_LABEL__", logLabel, 1)
	return strings.Replace(data, "ConfClientPathHolder", confClientPath, -1)
}

func buildMysqlConfData(c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) (string, error) {
	cfg := ini.Empty()
	sec := cfg.Section("mysqld")

	// my.cnf is mounted by pods still on the data-plane version; spec.mysqlVersion may already target an upgrade.
	v := mysqlConfVersion(c, cluster, sts)
	prof := mysqlversioning.ProfileFor(v)

	if prof.UseMySQL5xConfigs() {
		addKVConfigsToSection(sec, convertMapToKVConfig(mysql5xConfigs))
	} else if prof.UseMySQL8xConfigs() {
		addKVConfigsToSection(sec, convertMapToKVConfig(mysql8xConfigs))
		if prof.UseMySQL80AuthPlugin() {
			addKVConfigsToSection(sec, convertMapToKVConfig(mysql80AuthPluginConfig))
		}
	}

	// boolean configs (skip-host-cache removed in MySQL 8.0.30+ / Percona 8.0.30+)
	addBConfigsToSection(sec, mysqlMasterSlaveBooleanConfigsForVersion(v))
	// Official MySQL images default the client to /var/run/mysqld/mysqld.sock; Percona often uses the
	// datadir. We pin both server and client to the mounted data volume so probes, pt-heartbeat, and
	// ad-hoc `mysql` agree (errno 2 if client and server paths differ).
	opKV := MysqlKVConfigsForVersion(v)
	effectiveSocket := path.Join(DataVolumeMountPath, "mysql.sock")
	if _, userSet := cluster.Spec.MysqlConf["socket"]; !userSet {
		opKV["socket"] = effectiveSocket
	} else {
		sv := cluster.Spec.MysqlConf["socket"]
		effectiveSocket = (&sv).String()
	}
	addKVConfigsToSection(sec, convertMapToKVConfig(opKV), cluster.Spec.MysqlConf)

	clientSec, err := cfg.NewSection("client")
	if err != nil {
		return "", err
	}
	if _, err = clientSec.NewKey("socket", effectiveSocket); err != nil {
		return "", err
	}

	// include configs from /etc/mysql/conf.d/*.cnf
	_, err = sec.NewBooleanKey(fmt.Sprintf("!includedir %s", ConfDPath))
	if err != nil {
		return "", err
	}

	data, err := writeConfigs(cfg)
	if err != nil {
		return "", err
	}

	return data, nil

}

// mysqlConfVersion follows RolloutMySQLVersion so my.cnf tracks the StatefulSet rollout gate.
func mysqlConfVersion(c client.Client, cluster *mysqlcluster.MysqlCluster, sts *apps.StatefulSet) semver.Version {
	if c != nil {
		return versionupgrade.RolloutMySQLVersion(context.Background(), c, cluster, sts)
	}
	return cluster.EffectiveVersion(sts)
}

func convertMapToKVConfig(m map[string]string) map[string]intstr.IntOrString {
	config := make(map[string]intstr.IntOrString)

	for key, value := range m {
		config[key] = intstr.Parse(value)
	}

	return config
}

// helper function to add a map[string]string to a ini.Section
func addKVConfigsToSection(s *ini.Section, extraMysqld ...map[string]intstr.IntOrString) {
	for _, extra := range extraMysqld {
		keys := []string{}
		for key := range extra {
			keys = append(keys, key)
		}

		// sort keys
		sort.Strings(keys)

		for _, k := range keys {
			value := extra[k]
			if _, err := s.NewKey(k, value.String()); err != nil {
				log.Error(err, "failed to add key to config section", "key", k, "value", extra[k], "section", s)
			}
		}
	}
}

// helper function to add a string to a ini.Section
func addBConfigsToSection(s *ini.Section, boolConfigs ...[]string) {
	for _, extra := range boolConfigs {
		keys := []string{}
		keys = append(keys, extra...)

		// sort keys
		sort.Strings(keys)

		for _, k := range keys {
			if _, err := s.NewBooleanKey(k); err != nil {
				log.Error(err, "failed to add boolean key to config section", "key", k)
			}
		}
	}
}

// helper function to write to string ini.File
// nolint: interfacer
func writeConfigs(cfg *ini.File) (string, error) {
	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MysqlKVConfigsForVersion returns mysqld key/value defaults for the given server version.
func MysqlKVConfigsForVersion(v semver.Version) map[string]string {
	return mysqlversioning.OperatorKVForVersion(v)
}

var mysql5xConfigs = map[string]string{
	"query-cache-type": "0",
	"query-cache-size": "0",
	"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER," +
		"NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",

	"expire-logs-days": "14",
}

var mysql8xConfigs = map[string]string{
	"sql-mode": "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION," +
		"NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY",

	"binlog_expire_logs_seconds": "1209600", // 14 days = 14 * 24 * 60 * 60
}

// mysql80AuthPluginConfig is not applied on MySQL 8.4+ where mysql_native_password is unavailable.
var mysql80AuthPluginConfig = map[string]string{
	"default-authentication-plugin": "mysql_native_password",
}

func mysqlMasterSlaveBooleanConfigsForVersion(v semver.Version) []string {
	out := []string{
		// Safety
		"skip-name-resolve",
	}
	if !mysqlversion.AtLeastMySQL8030(v) {
		out = append(out, "skip-host-cache")
	}
	return out
}
