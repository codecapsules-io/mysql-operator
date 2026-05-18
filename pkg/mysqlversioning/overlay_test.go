/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"

	"github.com/bitpoke/mysql-operator/pkg/options"
)

func TestProfilesWithOverlay_prependMatchesBeforeBuiltin(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "overlay.yaml")
	content := `prependProfiles:
  - name: test-10x
    semverRange: ">=10.0.0 <11.0.0"
    baseProfile: ` + ProfilePercona97.String() + `
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	profs, err := ProfilesWithOverlay(p)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(profs)
	v := semver.MustParse("10.1.0")
	got := reg.MustResolve(v)
	if got.Name() != "test-10x" {
		t.Fatalf("expected overlay name test-10x, got %q", got.Name())
	}
	if got.SidecarProfileKey() != SidecarPercona97.String() {
		t.Fatalf("expected sidecar key from %s base, got %q", ProfilePercona97, got.SidecarProfileKey())
	}
	v80 := semver.MustParse("8.0.20")
	if reg.MustResolve(v80).Name() != ProfilePercona80.String() {
		t.Fatalf("8.0.20 should still hit %s, got %q", ProfilePercona80, reg.MustResolve(v80).Name())
	}
}

func TestBuiltinProfileSidecarKeys(t *testing.T) {
	cases := []struct {
		ver  string
		want SidecarProfileKey
	}{
		{"5.7.35", SidecarPercona57},
		{"8.0.20", SidecarPercona80},
		{"8.4.0", SidecarPercona84},
		{"9.7.0", SidecarPercona97},
	}
	reg := NewRegistry(BuiltinProfiles())
	for _, tc := range cases {
		v := semver.MustParse(tc.ver)
		if got := reg.MustResolve(v).SidecarProfileKey(); got != tc.want.String() {
			t.Fatalf("version %s: want sidecar key %q, got %q", tc.ver, tc.want, got)
		}
	}
}

func TestInitDefaultAndReload(t *testing.T) {
	opt := options.GetOptions()
	if err := InitDefault(opt, ""); err != nil {
		t.Fatal(err)
	}
	if ProfileFor(semver.MustParse("8.4.0")).Name() != ProfilePercona84.String() {
		t.Fatalf("expected %s after InitDefault", ProfilePercona84)
	}
	if err := Reload(opt, ""); err != nil {
		t.Fatal(err)
	}
	if ProfileFor(semver.MustParse("8.4.0")).Name() != ProfilePercona84.String() {
		t.Fatalf("expected %s after Reload", ProfilePercona84)
	}
}

func TestRegistryReplaceProfiles(t *testing.T) {
	reg := NewRegistry(BuiltinProfiles())
	if reg.MustResolve(semver.MustParse("8.4.0")).Name() != ProfilePercona84.String() {
		t.Fatal("builtin 8.4 profile missing")
	}
	reg.ReplaceProfiles([]Profile{profilePercona57{}, profileFallback{}})
	if reg.MustResolve(semver.MustParse("8.4.0")).Name() != ProfileFallbackUnknown.String() {
		t.Fatalf("after replace, 8.4 should hit %s: got %s", ProfileFallbackUnknown, reg.MustResolve(semver.MustParse("8.4.0")).Name())
	}
}
