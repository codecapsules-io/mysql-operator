/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package sidecar

import (
	"testing"

	"github.com/bitpoke/mysql-operator/pkg/util/constants"
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

func TestGetHeartbeatClientConfigs_usesUnixSocket(t *testing.T) {
	t.Parallel()
	cfg, err := getHeartbeatClientConfigs(constants.HeartBeatMySQLUser, "secret")
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
