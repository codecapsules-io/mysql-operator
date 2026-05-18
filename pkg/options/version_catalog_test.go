/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
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

9.7.0=percona/ps:9.7
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMySQLVersionCatalogFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["8.4.0"] != "percona/ps:8.4" || m["9.7.0"] != "percona/ps:9.7" {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestValidateLoadsCatalog(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.props")
	if err := os.WriteFile(p, []byte("9.7.0=img:9\n"), 0644); err != nil {
		t.Fatal(err)
	}
	o := &Options{
		MySQLVersionCatalogFile: p,
	}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	img, ok := o.MysqlImageFromCatalog("9.7.0")
	if !ok || img != "img:9" {
		t.Fatalf("got ok=%v img=%q", ok, img)
	}
}
