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
	"testing"

	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

func TestGetClientConfigs_includesGetServerPublicKey(t *testing.T) {
	t.Parallel()
	cfg, err := getClientConfigs(constants.HeartBeatMySQLUser, "secret")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := cfg.GetSection("client")
	if err != nil {
		t.Fatal(err)
	}
	if !sec.HasKey("get-server-public-key") {
		t.Fatal("expected get-server-public-key in [client]")
	}
	if sec.Key("get-server-public-key").String() != "1" {
		t.Fatalf("get-server-public-key: want 1 got %q", sec.Key("get-server-public-key").String())
	}
}

func TestGetSocketClientConfigs_usesUnixSocket(t *testing.T) {
	t.Parallel()
	cfg, err := getSocketClientConfigs(constants.HeartBeatMySQLUser, "secret")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := cfg.GetSection("client")
	if err != nil {
		t.Fatal(err)
	}
	wantSocket := constants.DataVolumeMountPath + "/mysql.sock"
	if !sec.HasKey("socket") || sec.Key("socket").String() != wantSocket {
		t.Fatalf("socket: want %q", wantSocket)
	}
	if sec.HasKey("host") || sec.HasKey("port") {
		t.Fatal("heartbeat client should not set host/port when using socket")
	}
	if sec.HasKey("get-server-public-key") {
		t.Fatal("heartbeat client should not rely on get-server-public-key (Perl DBD::mysql); use socket")
	}
}
