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
