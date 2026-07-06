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
	"fmt"
	"sync"

	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/codecapsules-io/mysql-operator/pkg/util/mysqlversion"
)

// Registry resolves a semver to a Profile (first registered match wins; register fallback last).
type Registry struct {
	mu       sync.RWMutex
	profiles []Profile
}

// NewRegistry builds a registry from an ordered profile list.
func NewRegistry(profiles []Profile) *Registry {
	cp := make([]Profile, len(profiles))
	copy(cp, profiles)
	return &Registry{profiles: cp}
}

// Resolve returns the first profile matching v.
func (r *Registry) Resolve(v semver.Version) (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.profiles {
		if p.Matches(v) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no mysql versioning profile matches %s", v.String())
}

// MustResolve returns the first matching profile. Builtin registries always include profileFallback last.
func (r *Registry) MustResolve(v semver.Version) Profile {
	p, err := r.Resolve(v)
	if err != nil {
		return profileFallback{}
	}
	return p
}

type profileFallback struct{}

func (profileFallback) Name() string { return ProfileFallbackUnknown.String() }

func (profileFallback) Matches(semver.Version) bool { return true }

func (profileFallback) MySQLOperatorKV(v semver.Version) map[string]string {
	return OperatorKVCommon(v, false)
}

func (profileFallback) UseMySQL5xConfigs() bool { return true }
func (profileFallback) UseMySQL8xConfigs() bool { return false }
func (profileFallback) UseMySQL80AuthPlugin() bool {
	return false
}

func (profileFallback) Replication() ReplicationDialect { return MasterSlaveReplication() }
func (profileFallback) GrantHints() GrantHints          { return masterSlaveGrantHintsMySQL57() }
func (profileFallback) SidecarProfileKey() string       { return SidecarPercona57.String() }

func (profileFallback) WantsPerconaInitContainer(semver.Version) bool {
	return false
}

func (profileFallback) Validate(spec *api.MysqlClusterSpec) error {
	return DefaultValidate(spec)
}

func (profileFallback) PodSecurityHints(perconaServerImage bool) PodSecurityHints {
	_ = perconaServerImage
	return PodSecurityLegacy999()
}

func (profileFallback) InnoDBOperatorLogSizing(v semver.Version, perFileBytes int64) (string, int64) {
	if mysqlversion.AtLeastMySQL8030(v) {
		return "innodb-redo-log-capacity", 2 * perFileBytes
	}
	return "innodb-log-file-size", perFileBytes
}
