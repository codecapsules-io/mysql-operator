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
package options

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMySQLVersionCatalogFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "catalog.properties")
	content := `
# comment
8.4.0=percona/ps:8.4

8.4.8=percona/ps:8.4.8
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMySQLVersionCatalogFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["8.4.0"] != "percona/ps:8.4" || m["8.4.8"] != "percona/ps:8.4.8" {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestValidateLoadsCatalog(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.props")
	if err := os.WriteFile(p, []byte("8.4.8=img:84\n"), 0644); err != nil {
		t.Fatal(err)
	}
	o := &Options{
		MySQLVersionCatalogFile: p,
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	img, ok := o.MysqlImageFromCatalog("8.4.8")
	if !ok || img != "img:84" {
		t.Fatalf("got ok=%v img=%q", ok, img)
	}
}
