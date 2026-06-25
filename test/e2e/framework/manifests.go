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
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

const operatorStatefulSetName = "mysql-operator"

func kubectl(args ...string) error {
	if TestContext.KubeContext != "" {
		args = append(args, "--context", TestContext.KubeContext)
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ApplyOperatorManifests(ns string) {
	base := TestContext.OperatorManifestsPath
	Expect(kubectl("apply", "-k", base+"/crds")).Should(Succeed())
	Expect(kubectl("apply", "-k", base+"/operator")).Should(Succeed())

	// Do not start pods with release images; patch local CI images first.
	Expect(kubectl("scale", "statefulset/"+operatorStatefulSetName, "-n", ns, "--replicas=0")).Should(Succeed())
	waitOperatorPodsGone(ns)

	cfg, err := LoadConfig()
	Expect(err).NotTo(HaveOccurred())
	client, err := clientset.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	patchOperatorStatefulSet(client, ns)

	Expect(kubectl("scale", "statefulset/"+operatorStatefulSetName, "-n", ns, "--replicas=1")).Should(Succeed())
	waitOperatorRollout(ns)
}

func RemoveOperatorManifests(ns string) {
	base := TestContext.OperatorManifestsPath
	_ = kubectl("delete", "-k", base+"/operator", "-n", ns, "--ignore-not-found", "--wait=false")
}

func waitOperatorPodsGone(ns string) {
	_ = kubectl(
		"wait", "--for=delete", "pod",
		"-l", "app.kubernetes.io/name=mysql-operator",
		"-n", ns, "--timeout=120s",
	)
}

func waitOperatorRollout(ns string) {
	err := kubectl(
		"rollout", "status", "statefulset/"+operatorStatefulSetName,
		"-n", ns, "--timeout=10m",
	)
	if err != nil {
		dumpOperatorDiagnostics(ns)
	}
	Expect(err).Should(Succeed())
}

func dumpOperatorDiagnostics(ns string) {
	Logf("operator rollout failed; dumping cluster diagnostics")
	_ = kubectl("get", "pods", "-n", ns, "-o", "wide")
	_ = kubectl("describe", "pod", "-n", ns, "-l", "app.kubernetes.io/name=mysql-operator")
	_ = kubectl("get", "events", "-n", ns, "--sort-by=.lastTimestamp")
}

func patchOperatorStatefulSet(client clientset.Interface, ns string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var ss *appsv1.StatefulSet
	Eventually(func() error {
		var err error
		ss, err = client.AppsV1().StatefulSets(ns).Get(ctx, operatorStatefulSetName, metav1.GetOptions{})
		return err
	}).Should(Succeed())

	for i := range ss.Spec.Template.Spec.Containers {
		switch ss.Spec.Template.Spec.Containers[i].Name {
		case "operator":
			ss.Spec.Template.Spec.Containers[i].Image = TestContext.OperatorImage
			ss.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullIfNotPresent
			ss.Spec.Template.Spec.Containers[i].Args = patchOperatorArgs(ss.Spec.Template.Spec.Containers[i].Args)
			if ss.Spec.Template.Spec.Containers[i].ReadinessProbe != nil {
				ss.Spec.Template.Spec.Containers[i].ReadinessProbe.InitialDelaySeconds = 15
			}
		case "orchestrator":
			ss.Spec.Template.Spec.Containers[i].Image = TestContext.OrchestratorImage
			ss.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullIfNotPresent
			if ss.Spec.Template.Spec.Containers[i].ReadinessProbe != nil {
				ss.Spec.Template.Spec.Containers[i].ReadinessProbe.InitialDelaySeconds = 60
			}
		}
	}

	_, err := client.AppsV1().StatefulSets(ns).Update(ctx, ss, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func patchOperatorArgs(args []string) []string {
	args = setArg(args, "--sidecar-image", TestContext.SidecarMysql57Image)
	args = setArg(args, "--sidecar-mysql8-image", TestContext.SidecarMysql8Image)
	args = setArg(args, "--sidecar-mysql84-image", TestContext.SidecarMysql84Image)
	args = setArg(args, "--metrics-exporter-image", TestContext.MetricsExporterImage)
	args = setArg(args, "--image-pull-policy", "IfNotPresent")
	for version, image := range KindE2eMysqlVersionOverrides() {
		args = appendMysqlVersionImage(args, version, image)
	}
	return ensureFlag(args, "--debug")
}

func appendMysqlVersionImage(args []string, version, image string) []string {
	if image == "" {
		return args
	}
	prefix := "--mysql-versions-to-image=" + version + "="
	out := make([]string, 0, len(args)+1)
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			continue
		}
		out = append(out, arg)
	}
	out = append(out, prefix+image)
	return out
}

func ensureFlag(args []string, name string) []string {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return args
		}
	}
	return append(args, name)
}

func setArg(args []string, name, value string) []string {
	prefix := name + "="
	out := make([]string, 0, len(args)+1)
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) || arg == name {
			continue
		}
		out = append(out, arg)
	}
	if value != "" {
		out = append(out, prefix+value)
	}
	return out
}
