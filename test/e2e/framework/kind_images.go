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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/codecapsules-io/mysql-operator/pkg/util/constants"
)

const defaultKindClusterName = "mysql-operator"

// KindE2eMysqlImage returns the Kind-local Percona server image for a profile suffix (e.g. percona-5.7).
func KindE2eMysqlImage(profile string) string {
	return fmt.Sprintf("%s/%s:%s", TestContext.KindE2eRegistry, profile, TestContext.KindE2eTag)
}

// RequiredKindImages lists every image reference cluster and operator pods use in Kind e2e.
func RequiredKindImages() []string {
	images := []string{
		TestContext.OperatorImage,
		TestContext.OrchestratorImage,
		TestContext.SidecarMysql57Image,
		TestContext.SidecarMysql8Image,
		TestContext.MetricsExporterImage,
	}
	if TestContext.SidecarMysql84Image != "" {
		images = append(images, TestContext.SidecarMysql84Image)
	}
	if TestContext.KindE2eRegistry != "" && TestContext.KindE2eTag != "" {
		images = append(images,
			KindE2eMysqlImage("percona-5.7"),
			KindE2eMysqlImage("percona-8.0"),
			KindE2eMysqlImage("percona-8.4"),
		)
	}
	return images
}

// VerifyKindImages fails fast when a preloaded image is missing from the kind node.
func VerifyKindImages() {
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
	for _, ref := range RequiredKindImages() {
		if ref == "" {
			continue
		}
		if !imageRefPresentInCrictl(crictlOut, ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		table, _ := exec.Command("docker", "exec", node, "crictl", "images").CombinedOutput()
		Failf("kind image preflight: missing %d image(s) on %s: %s\n\ncrictl images:\n%s",
			len(missing), node, strings.Join(missing, ", "), string(table))
	}
	Logf("kind image preflight: all %d required images present on %s", len(RequiredKindImages()), node)
}

// KindE2eMysqlVersionOverrides returns mysql-versions-to-image entries for patchOperatorArgs.
func KindE2eMysqlVersionOverrides() map[string]string {
	if TestContext.KindE2eRegistry == "" || TestContext.KindE2eTag == "" {
		return nil
	}
	return map[string]string{
		constants.MySQLVersion5735.String(): KindE2eMysqlImage("percona-5.7"),
		constants.MySQLVersion8020.String(): KindE2eMysqlImage("percona-8.0"),
		constants.MySQLVersion848.String():  KindE2eMysqlImage("percona-8.4"),
	}
}
