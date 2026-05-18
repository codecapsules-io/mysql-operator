/*
Copyright 2026 Pressinfra SRL

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package mysqlversioning

// PodSecurityHints configures StatefulSet pod.spec.securityContext (fsGroup / optional runAsUser)
// and optional per-container overrides for the Percona server image.
// RunAsUser nil at pod level means the field is omitted so sidecars can keep image USER 999.
// MysqlRunAsUser/MysqlRunAsGroup, when set, apply only to the mysqld (and mysql-init-only) containers
// so the server runs as UID/GID 1001 even if policy or stale merges leave pod.runAsUser at 999.
type PodSecurityHints struct {
	FSGroup   int64
	RunAsUser *int64
	// MysqlRunAsUser, if non-nil, is applied as securityContext on the mysql and mysql-init-only containers only.
	MysqlRunAsUser *int64
	// MysqlRunAsGroup, if non-nil, is applied with MysqlRunAsUser on those containers (typically 1001 for PS 8.4+).
	MysqlRunAsGroup *int64
}

const (
	operatorLegacyMySQLUID int64 = 999
	perconaUIDGID1001      int64 = 1001
)

// PodSecurityLegacy999 matches historical operator behaviour: force UID/GID 999 for every container.
func PodSecurityLegacy999() PodSecurityHints {
	u := operatorLegacyMySQLUID
	return PodSecurityHints{FSGroup: u, RunAsUser: &u}
}

// PodSecurityPerconaUID1001VolumeGroup sets fsGroup for shared volumes; omits pod runAsUser so sidecars
// stay on 999. Forces the mysql containers to UID/GID 1001 to match official Percona Server 8.4+ images
// and to survive pod-level runAsUser forced to 999 (policy or merge artifacts).
func PodSecurityPerconaUID1001VolumeGroup() PodSecurityHints {
	fg := perconaUIDGID1001
	u := perconaUIDGID1001
	g := perconaUIDGID1001
	return PodSecurityHints{
		FSGroup:         fg,
		RunAsUser:       nil,
		MysqlRunAsUser:  &u,
		MysqlRunAsGroup: &g,
	}
}
