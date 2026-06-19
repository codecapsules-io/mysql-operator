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

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"

	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
	"github.com/codecapsules-io/mysql-operator/pkg/util/semver"
)

// MySQLVersionEnv is set on mysql pods (StatefulSet template) to record the server line they run.
const MySQLVersionEnv = "MY_MYSQL_VERSION"

// DesiredVersion resolves spec.mysqlVersion (alias → operator default) to a semver.
func DesiredVersion(specVersion string) semver.Version {
	version := specVersion
	if version == "" {
		version = constants.MySQLDefaultVersion
	}
	if v, ok := constants.MySQLTagsToSemVer[version]; ok {
		version = v
	}
	sv, _ := semver.Make(version)
	return sv
}

// AppliedDataPlaneVersion parses status.appliedMysqlVersion after a completed rollout.
func AppliedDataPlaneVersion(statusApplied string) semver.Version {
	if statusApplied == "" {
		return semver.Zero
	}
	v, err := semver.Parse(statusApplied)
	if err != nil {
		return semver.Zero
	}
	return v
}

// SemVerFromStatefulSet reads MY_MYSQL_VERSION from the StatefulSet pod template, then the mysql
// container image tag (legacy clusters may lack the env var).
func SemVerFromStatefulSet(sts *apps.StatefulSet) semver.Version {
	if sts == nil {
		return semver.Zero
	}
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == "mysql" {
			return semVerFromMysqlContainer(c)
		}
	}
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == "mysql-init-only" {
			return semVerFromMysqlContainer(c)
		}
	}
	return semver.Zero
}

// SemVerFromPod reads MY_MYSQL_VERSION from a running mysql pod, then the container image tag.
func SemVerFromPod(pod *core.Pod) semver.Version {
	if pod == nil {
		return semver.Zero
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == "mysql" {
			return semVerFromMysqlContainer(c)
		}
	}
	return semver.Zero
}

func semVerFromMysqlContainer(c core.Container) semver.Version {
	for _, e := range c.Env {
		if e.Name == MySQLVersionEnv && e.Value != "" {
			if v, err := semver.Parse(e.Value); err == nil {
				return v
			}
		}
	}
	return semVerFromImageRef(c.Image)
}

// ParseServerVersion parses a MySQL server version string (e.g. from SELECT VERSION() or image tags).
func ParseServerVersion(version string) (semver.Version, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return semver.Zero, fmt.Errorf("empty server version")
	}
	// SELECT VERSION() returns e.g. 8.0.34-26 — strip vendor suffix after the first dash.
	if dash := strings.Index(version, "-"); dash > 0 {
		if v, err := semver.Parse(version[:dash]); err == nil {
			return v, nil
		}
	}
	if v, err := semver.Parse(version); err == nil {
		return v, nil
	}
	v := semVerFromImageRef(version)
	if v.IsZero() {
		return semver.Zero, fmt.Errorf("unrecognized server version %q", version)
	}
	return v, nil
}

func semVerFromImageRef(image string) semver.Version {
	if image == "" {
		return semver.Zero
	}
	tag := imageTag(image)
	if tag == "" || tag == "latest" {
		return semver.Zero
	}
	if mapped, ok := constants.MySQLTagsToSemVer[tag]; ok {
		if v, err := semver.Parse(mapped); err == nil {
			return v
		}
	}
	if dash := strings.Index(tag, "-"); dash > 0 {
		if v, err := semver.Parse(tag[:dash]); err == nil {
			return v
		}
	}
	if v, err := semver.Parse(tag); err == nil {
		return v
	}
	if v, err := semver.Make(tag); err == nil {
		return v
	}
	return semver.Zero
}

func imageTag(image string) string {
	if at := strings.LastIndex(image, "@"); at >= 0 && strings.Contains(image[at:], "sha256:") {
		return ""
	}
	if idx := strings.LastIndex(image, ":"); idx >= 0 {
		return image[idx+1:]
	}
	return ""
}
