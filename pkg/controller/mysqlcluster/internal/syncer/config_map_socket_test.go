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
package mysqlcluster

import (
	"strings"
	"testing"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	icluster "github.com/bitpoke/mysql-operator/pkg/internal/mysqlcluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildMysqlConfData_socketOnDataVolume(t *testing.T) {
	t.Parallel()
	c := icluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.4.0",
			SecretName:   "sec",
		},
	})
	data, err := buildMysqlConfData(c)
	if err != nil {
		t.Fatal(err)
	}
	want := DataVolumeMountPath + "/mysql.sock"
	if !strings.Contains(data, "[mysqld]") || !strings.Contains(data, want) {
		t.Fatalf("mysqld socket: want path %q in my.cnf, got:\n%s", want, data)
	}
	if !strings.Contains(data, "[client]") || !strings.Contains(data, want) {
		t.Fatalf("[client] socket: want %q in my.cnf, got:\n%s", want, data)
	}
}

func TestBuildMysqlConfData_socketUserOverride(t *testing.T) {
	t.Parallel()
	custom := "/tmp/custom.sock"
	c := icluster.New(&api.MysqlCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: api.MysqlClusterSpec{
			MysqlVersion: "8.0.20",
			SecretName:   "sec",
			MysqlConf: map[string]intstr.IntOrString{
				"socket": intstr.FromString(custom),
			},
		},
	})
	data, err := buildMysqlConfData(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, custom) {
		t.Fatalf("expected user socket in output, got:\n%s", data)
	}
	if strings.Count(data, custom) < 2 {
		t.Fatalf("expected socket in both [mysqld] and [client], got:\n%s", data)
	}
}
