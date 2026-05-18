/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

import "strings"

// ProfileName is the stable identifier returned by Profile.Name() for built-in profiles.
type ProfileName string

const (
	ProfilePercona57       ProfileName = "percona-5.7"
	ProfilePercona80       ProfileName = "percona-8.0"
	ProfilePercona84       ProfileName = "percona-8.4"
	ProfilePercona97       ProfileName = "percona-9.7"
	ProfileFallbackUnknown ProfileName = "fallback-unknown"
)

// String implements fmt.Stringer.
func (n ProfileName) String() string {
	return string(n)
}

// SidecarProfileKey selects which operator sidecar image flag to use for a profile line.
type SidecarProfileKey string

const (
	SidecarPercona57 SidecarProfileKey = "percona-57"
	SidecarPercona80 SidecarProfileKey = "percona-80"
	SidecarPercona84 SidecarProfileKey = "percona-84"
	SidecarPercona97 SidecarProfileKey = "percona-97"
)

// String implements fmt.Stringer.
func (k SidecarProfileKey) String() string {
	return string(k)
}

// profileUpgradeOrder lists built-in profiles in supported server upgrade order (one LTS step at a time).
var profileUpgradeOrder = []ProfileName{
	ProfilePercona57,
	ProfilePercona80,
	ProfilePercona84,
	ProfilePercona97,
	ProfileFallbackUnknown,
}

func profileUpgradeIndex(name string) (int, bool) {
	for i, n := range profileUpgradeOrder {
		if n.String() == name {
			return i, true
		}
	}
	return 0, false
}

// BuiltinProfileNamesForHelp returns built-in ProfileName values for error messages and docs.
func BuiltinProfileNamesForHelp() string {
	names := make([]string, len(profileUpgradeOrder))
	for i, n := range profileUpgradeOrder {
		names[i] = n.String()
	}
	return strings.Join(names, ", ")
}
