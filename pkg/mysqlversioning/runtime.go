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
	"sync"

	"github.com/blang/semver"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/options"
)

// Runtime bundles version resolution and image resolution for the operator process.
type Runtime struct {
	Registry *Registry
	Resolver *ImageResolver
}

var (
	defaultRuntimeMu sync.RWMutex
	defaultRuntime   *Runtime
)

// InitDefault wires the process-wide runtime used by controllers and the sidecar.
// Call once from main after options.Validate(), and from the sidecar after parsing env.
func InitDefault(opt *options.Options, overlayPath string) error {
	profs, err := ProfilesWithOverlay(overlayPath)
	if err != nil {
		return err
	}
	reg := NewRegistry(profs)
	res := NewImageResolver(opt)
	rt := &Runtime{Registry: reg, Resolver: res}
	defaultRuntimeMu.Lock()
	defaultRuntime = rt
	defaultRuntimeMu.Unlock()
	return nil
}

// Reload rebuilds the registry from built-ins plus overlay file and refreshes the image resolver.
func Reload(opt *options.Options, overlayPath string) error {
	profs, err := ProfilesWithOverlay(overlayPath)
	if err != nil {
		return err
	}
	defaultRuntimeMu.Lock()
	defer defaultRuntimeMu.Unlock()
	if defaultRuntime == nil {
		defaultRuntime = &Runtime{
			Registry: NewRegistry(profs),
			Resolver: NewImageResolver(opt),
		}
		return nil
	}
	defaultRuntime.Registry.ReplaceProfiles(profs)
	defaultRuntime.Resolver = NewImageResolver(opt)
	return nil
}

// Default returns the initialized runtime or nil if InitDefault was not called.
func Default() *Runtime {
	defaultRuntimeMu.RLock()
	defer defaultRuntimeMu.RUnlock()
	return defaultRuntime
}

// ProfileFor resolves the profile for a semver (process registry when set, otherwise built-ins only).
func ProfileFor(v semver.Version) Profile {
	if rt := Default(); rt != nil {
		return rt.Registry.MustResolve(v)
	}
	return NewRegistry(BuiltinProfiles()).MustResolve(v)
}

// ReplicationFor returns the replication dialect for a semver.
func ReplicationFor(v semver.Version) ReplicationDialect {
	return ProfileFor(v).Replication()
}

// ServerImage resolves the server image using Default() or a fresh resolver when unset.
func ServerImage(opt *options.Options, ver semver.Version, spec *api.MysqlClusterSpec) (string, error) {
	if Default() != nil {
		return Default().Resolver.ServerImage(ver, spec)
	}
	return NewImageResolver(opt).ServerImage(ver, spec)
}

// SidecarImageFor resolves the sidecar image for a cluster semver and optional spec.sidecarImage override.
func SidecarImageFor(ver semver.Version, spec *api.MysqlClusterSpec, sidecarOverride string) string {
	if sidecarOverride != "" {
		return sidecarOverride
	}
	opt := options.GetOptions()
	if rt := Default(); rt != nil {
		p := rt.Registry.MustResolve(ver)
		return rt.Resolver.SidecarImage(p.SidecarProfileKey())
	}
	reg := NewRegistry(BuiltinProfiles())
	p := reg.MustResolve(ver)
	return NewImageResolver(opt).SidecarImage(p.SidecarProfileKey())
}
