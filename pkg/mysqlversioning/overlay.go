/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import (
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	yaml "gopkg.in/yaml.v2"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
)

// overlayFile is the declarative schema for optional profile prepends (merged before built-ins).
type overlayFile struct {
	PrependProfiles []struct {
		Name        string `yaml:"name"`
		SemverRange string `yaml:"semverRange"`
		BaseProfile string `yaml:"baseProfile"`
	} `yaml:"prependProfiles"`
}

type overlayRangeProfile struct {
	displayName string
	rng         semver.Range
	base        Profile
}

func (o overlayRangeProfile) Name() string { return o.displayName }

func (o overlayRangeProfile) Matches(v semver.Version) bool {
	return o.rng(v)
}

func (o overlayRangeProfile) MySQLOperatorKV(v semver.Version) map[string]string {
	return o.base.MySQLOperatorKV(v)
}

func (o overlayRangeProfile) UseMySQL5xConfigs() bool { return o.base.UseMySQL5xConfigs() }

func (o overlayRangeProfile) UseMySQL8xConfigs() bool { return o.base.UseMySQL8xConfigs() }

func (o overlayRangeProfile) UseMySQL80AuthPlugin() bool { return o.base.UseMySQL80AuthPlugin() }

func (o overlayRangeProfile) Replication() ReplicationDialect { return o.base.Replication() }

func (o overlayRangeProfile) GrantHints() GrantHints { return o.base.GrantHints() }

func (o overlayRangeProfile) SidecarProfileKey() string { return o.base.SidecarProfileKey() }

func (o overlayRangeProfile) WantsPerconaInitContainer(v semver.Version) bool {
	return o.base.WantsPerconaInitContainer(v)
}

func (o overlayRangeProfile) Validate(spec *api.MysqlClusterSpec) error {
	return o.base.Validate(spec)
}

func (o overlayRangeProfile) PodSecurityHints(perconaServerImage bool) PodSecurityHints {
	return o.base.PodSecurityHints(perconaServerImage)
}

func (o overlayRangeProfile) InnoDBOperatorLogSizing(v semver.Version, perFileBytes int64) (string, int64) {
	return o.base.InnoDBOperatorLogSizing(v, perFileBytes)
}

func builtinProfilesByName() map[string]Profile {
	out := map[string]Profile{}
	for _, p := range []Profile{
		profilePercona97{},
		profilePercona84{},
		profilePercona80{},
		profilePercona57{},
		profileFallback{},
	} {
		out[p.Name()] = p
	}
	return out
}

// parseProfileOverlayFile returns profiles to prepend before BuiltinProfiles(), or nil if none.
func parseProfileOverlayFile(path string) ([]Profile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var f overlayFile
	if err := yaml.UnmarshalStrict(data, &f); err != nil {
		return nil, fmt.Errorf("mysql profile overlay %q: %w", path, err)
	}
	if len(f.PrependProfiles) == 0 {
		return nil, nil
	}
	byName := builtinProfilesByName()
	var prepended []Profile
	for _, row := range f.PrependProfiles {
		if row.Name == "" || row.SemverRange == "" || row.BaseProfile == "" {
			return nil, fmt.Errorf("mysql profile overlay %q: each prependProfiles entry needs name, semverRange, baseProfile", path)
		}
		base, ok := byName[row.BaseProfile]
		if !ok {
			return nil, fmt.Errorf("mysql profile overlay %q: unknown baseProfile %q (use %s)", path, row.BaseProfile, BuiltinProfileNamesForHelp())
		}
		rng, err := semver.ParseRange(row.SemverRange)
		if err != nil {
			return nil, fmt.Errorf("mysql profile overlay %q: semverRange %q: %w", path, row.SemverRange, err)
		}
		prepended = append(prepended, overlayRangeProfile{
			displayName: row.Name,
			rng:         rng,
			base:        base,
		})
	}
	return prepended, nil
}

// ProfilesWithOverlay returns BuiltinProfiles() prefixed by overlay definitions from path when present.
func ProfilesWithOverlay(overlayPath string) ([]Profile, error) {
	prepend, err := parseProfileOverlayFile(overlayPath)
	if err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(prepend)+len(BuiltinProfiles()))
	out = append(out, prepend...)
	out = append(out, BuiltinProfiles()...)
	return out, nil
}
