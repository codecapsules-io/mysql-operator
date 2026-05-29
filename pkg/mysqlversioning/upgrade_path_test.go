/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"testing"

	"github.com/blang/semver"
)

func TestNeedsAuthPluginMigration(t *testing.T) {
	v80 := semver.MustParse("8.0.34")
	v84 := semver.MustParse("8.4.0")

	if !NeedsAuthPluginMigration(v80, v84) {
		t.Fatal("expected 8.0 -> 8.4 to require auth plugin migration")
	}
	if NeedsAuthPluginMigration(v80, v80) {
		t.Fatal("expected same version to skip auth migration")
	}
	if NeedsAuthPluginMigration(semver.Version{}, v84) {
		t.Fatal("expected empty current to skip auth migration")
	}
	if NeedsAuthPluginMigration(v84, semver.MustParse("8.4.8")) {
		t.Fatal("expected patch bump on 8.4 line to skip mysql_native_password migration")
	}
}
