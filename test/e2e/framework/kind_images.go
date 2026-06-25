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

package framework

import (
	"os"
	"os/exec"
	"strings"
)

const defaultKindClusterName = "mysql-operator"

// RequiredLocalKindImages lists operator images that must be preloaded via kind load docker-image.
// Public images (Percona server, mysqld-exporter) are pulled by kind nodes from Docker Hub.
func RequiredLocalKindImages() []string {
	images := []string{
		TestContext.OperatorImage,
		TestContext.OrchestratorImage,
		TestContext.SidecarMysql57Image,
		TestContext.SidecarMysql8Image,
	}
	if TestContext.SidecarMysql84Image != "" {
		images = append(images, TestContext.SidecarMysql84Image)
	}
	return images
}

// VerifyKindLocalImages fails fast when a locally built image is missing from the kind node.
func VerifyKindLocalImages() {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = defaultKindClusterName
	}
	node := clusterName + "-control-plane"

	out, err := exec.Command("docker", "exec", node, "crictl", "images", "-o", "json").CombinedOutput()
	if err != nil {
		Failf("kind image preflight: crictl on %s failed: %v\n%s", node, err, string(out))
	}
	crictlOut := string(out)

	var missing []string
	for _, ref := range RequiredLocalKindImages() {
		if ref == "" {
			continue
		}
		if !imageRefPresentInCrictl(crictlOut, ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		table, _ := exec.Command("docker", "exec", node, "crictl", "images").CombinedOutput()
		Failf("kind image preflight: missing %d locally loaded image(s) on %s: %s\n\ncrictl images:\n%s",
			len(missing), node, strings.Join(missing, ", "), string(table))
	}
	Logf("kind image preflight: all %d locally loaded images present on %s", len(RequiredLocalKindImages()), node)
}
