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
	"strings"

	"github.com/blang/semver"

	api "github.com/bitpoke/mysql-operator/pkg/apis/mysql/v1alpha1"
	"github.com/bitpoke/mysql-operator/pkg/options"
	"github.com/bitpoke/mysql-operator/pkg/util/constants"
)

// ImageResolver resolves server and sidecar container images from flags and spec.
type ImageResolver struct {
	opt *options.Options
}

// NewImageResolver constructs a resolver bound to operator options.
func NewImageResolver(opt *options.Options) *ImageResolver {
	return &ImageResolver{opt: opt}
}

// ServerImage resolves the MySQL server image for a cluster.
func (r *ImageResolver) ServerImage(ver semver.Version, spec *api.MysqlClusterSpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("nil MysqlCluster spec")
	}
	if len(spec.Image) != 0 {
		return spec.Image, nil
	}
	key := ver.String()
	if img, ok := r.opt.MySQLVersionImageOverride[key]; ok {
		return img, nil
	}
	if img, ok := r.opt.MysqlImageFromCatalog(key); ok {
		return img, nil
	}
	if img, ok := constants.MysqlImageVersions[key]; ok {
		return img, nil
	}
	return "", fmt.Errorf("no server image for mysql version %s", key)
}

// SidecarImage maps a profile sidecar key to the configured container image.
func (r *ImageResolver) SidecarImage(key string) string {
	switch SidecarProfileKey(key) {
	case SidecarPercona57:
		return r.opt.SidecarMysql57Image
	case SidecarPercona80:
		return r.opt.SidecarMysql8Image
	case SidecarPercona84:
		if r.opt.SidecarMysql84Image != "" {
			return r.opt.SidecarMysql84Image
		}
		return r.opt.SidecarMysql8Image
	default:
		return r.opt.SidecarMysql8Image
	}
}

// IsPerconaImage returns true if the resolved server image name suggests Percona.
func (r *ImageResolver) IsPerconaImage(ver semver.Version, spec *api.MysqlClusterSpec) bool {
	img, err := r.ServerImage(ver, spec)
	if err != nil || img == "" {
		return false
	}
	return strings.Contains(img, "percona")
}
