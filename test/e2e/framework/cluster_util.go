/*
Copyright 2018 Pressinfra SRL
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
	"fmt"
	"sort"
	"strings"
	"time"

	"database/sql"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	k8score "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"

	"github.com/codecapsules-io/mysql-operator/pkg/apis/domain"
	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	pf "github.com/codecapsules-io/mysql-operator/test/e2e/framework/portforward"
)

var (
	POLLING = 2 * time.Second
)

func (f *Framework) ClusterEventuallyCondition(cluster *api.MysqlCluster,
	condType api.ClusterConditionType, status corev1.ConditionStatus, timeout time.Duration) {
	Eventually(func() []api.ClusterCondition {
		key := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		if err := f.Client.Get(context.TODO(), key, cluster); err != nil {
			return nil
		}
		return cluster.Status.Conditions
	}, timeout, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing cluster '%s' for condition %s to be %s", cluster.Name, condType, status)

}

func (f *Framework) NodeEventuallyCondition(cluster *api.MysqlCluster, nodeName string,
	condType api.NodeConditionType, status corev1.ConditionStatus, timeout time.Duration) {
	Eventually(func() []api.NodeCondition {
		key := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		if err := f.Client.Get(context.TODO(), key, cluster); err != nil {
			return nil
		}

		for _, ns := range cluster.Status.Nodes {
			if ns.Name == nodeName {
				return ns.Conditions
			}
		}

		return nil
	}, timeout, POLLING).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(condType),
		"Status": Equal(status),
	})), "Testing node '%s' of the cluster '%s'", cluster.Name, nodeName)
}

func (f *Framework) ExecSQLOnNode(cluster *api.MysqlCluster, i int, user, password, query string) *sql.Rows {
	kubeCfg, err := LoadConfig()
	Expect(err).NotTo(HaveOccurred())

	podName := strings.Split(f.GetPodHostname(cluster, i), ".")[0]

	client := k8score.NewForConfigOrDie(kubeCfg).RESTClient()
	tunnel := pf.NewTunnel(client, kubeCfg, cluster.Namespace,
		podName,
		3306,
	)

	err = tunnel.ForwardPort()
	Expect(err).NotTo(HaveOccurred(), "Failed setting up port-forarding for pod: %s", podName)

	dsn := fmt.Sprintf("%s:%s@tcp(localhost:%d)/?timeout=10s&multiStatements=true", user, password, tunnel.Local)
	db, err := sql.Open("mysql", dsn)
	Expect(err).To(Succeed(), "Failed connection to mysql DSN: %s", dsn)

	rows, err := db.Query(query)
	Expect(err).To(Succeed(), "Query failed: %s", query)

	tunnel.Close()
	return rows
}

func (f *Framework) GetPodForNode(cluster *api.MysqlCluster, i int) *corev1.Pod {
	selector := labels.SelectorFromSet(cluster.GetLabels())
	podList, err := f.ClientSet.CoreV1().Pods(cluster.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed listing pods for cluster '%s'", cluster.Name)

	hostname := f.GetPodHostname(cluster, i)
	for _, pod := range podList.Items {
		if strings.Contains(hostname, pod.Name) {
			return &pod
		}
	}

	return nil
}

// GetPodHostname returns for an index the pod hostname of a cluster
func (f *Framework) GetPodHostname(cluster *api.MysqlCluster, p int) string {
	return fmt.Sprintf("%s-%d.%s.%s", GetNameForResource("sts", cluster), p,
		GetNameForResource("svc-headless", cluster),
		cluster.Namespace)
}

// GetNameForResource returns the name of the cluster resource, see the function
// definition for what name means.
func GetNameForResource(name string, cluster *api.MysqlCluster) string {
	switch name {
	case "sts":
		return fmt.Sprintf("%s-mysql", cluster.Name)
	case "svc-master":
		return fmt.Sprintf("%s-mysql-master", cluster.Name)
	case "svc-read":
		return fmt.Sprintf("%s-mysql", cluster.Name)
	case "svc-headless":
		return "mysql"
	default:
		return fmt.Sprintf("%s-mysql", cluster.Name)
	}
}

// HaveClusterCond is a helper func that returns a matcher to check for an existing condition in a ClusterCondition list.
func HaveClusterCond(condType api.ClusterConditionType, status corev1.ConditionStatus) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Conditions": ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(condType),
				"Status": Equal(status),
			})),
		})},
	))
}

func (f *Framework) RefreshClusterFn(cluster *api.MysqlCluster) func() *api.MysqlCluster {
	return func() *api.MysqlCluster {
		key := types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}
		c := &api.MysqlCluster{}
		f.Client.Get(context.TODO(), key, c)
		return c
	}
}

// HaveClusterRepliacs matcher for replicas
func HaveClusterReplicas(replicas int) gomegatypes.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"ReadyNodes": Equal(replicas),
		}),
	}))
}

var (
	testDBName    = "op_test_w"
	testTableName = "op_table"
)

func (f *Framework) WriteSQLTest(cluster *api.MysqlCluster, pod int, pw string) string {
	By("run write SQL test to cluster")

	// create database
	f.ExecSQLOnNode(cluster, pod, "root", pw,
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", testDBName),
	)

	// create table
	f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`USE %s; CREATE TABLE IF NOT EXISTS %s
           (k varchar(20) NOT NULL, v varchar(36) NOT NULL, PRIMARY KEY (k));`,
		testDBName, testTableName,
	))

	// insert data
	data := string(uuid.NewUUID())
	f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`USE %s; INSERT INTO %s (k, v) VALUES ("data", "%s")
                    ON DUPLICATE KEY UPDATE k="data", v="%[3]s";`,
		testDBName, testTableName, data,
	))

	return data
}

func (f *Framework) ReadSQLTest(cluster *api.MysqlCluster, pod int, pw string) string {
	By("run read SQL test")
	var data string

	rows := f.ExecSQLOnNode(cluster, pod, "root", pw, fmt.Sprintf(
		`SELECT v FROM %s.%s WHERE k="data"`,
		testDBName, testTableName,
	))
	defer rows.Close()

	if rows.Next() {
		rows.Scan(&data)
	}

	return data
}

// GetClusterLabels returns labels.Set for the given cluster
func GetClusterLabels(cluster *api.MysqlCluster) labels.Set {
	labels := labels.Set{
		domain.LabelCluster:      cluster.Name,
		"app.kubernetes.io/name": "mysql",
	}

	return labels
}

func (f *Framework) GetClusterPVCsFn(cluster *api.MysqlCluster) func() []corev1.PersistentVolumeClaim {
	return func() []corev1.PersistentVolumeClaim {
		pvcList := &corev1.PersistentVolumeClaimList{}
		lo := &client.ListOptions{
			Namespace:     cluster.Namespace,
			LabelSelector: labels.SelectorFromSet(GetClusterLabels(cluster)),
		}
		f.Client.List(context.TODO(), pvcList, lo)
		return pvcList.Items
	}
}

// LogClusterReadinessDiagnostics prints cluster status, workload objects, and recent events
// to help debug e2e readiness timeouts.
func (f *Framework) LogClusterReadinessDiagnostics(cluster *api.MysqlCluster, cl *api.MysqlCluster) {
	if cl == nil {
		cl = &api.MysqlCluster{}
		key := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
		if err := f.Client.Get(context.TODO(), key, cl); err != nil {
			Logf("cluster readiness diagnostics: failed to get MysqlCluster %s/%s: %v",
				cluster.Namespace, cluster.Name, err)
			return
		}
	}

	replicas := int32(0)
	if cluster.Spec.Replicas != nil {
		replicas = *cluster.Spec.Replicas
	}

	Logf("=== cluster readiness diagnostics: %s/%s ===", cluster.Namespace, cluster.Name)
	Logf("spec.replicas=%d status.readyNodes=%d appliedMysqlVersion=%q spec.mysqlVersion=%q spec.image=%q",
		replicas, cl.Status.ReadyNodes, cl.Status.AppliedMysqlVersion, cl.Spec.MysqlVersion, cl.Spec.Image)

	for _, cond := range cl.Status.Conditions {
		Logf("cluster condition %s=%s reason=%q message=%q",
			cond.Type, cond.Status, cond.Reason, cond.Message)
	}
	for _, node := range cl.Status.Nodes {
		Logf("status node %q master=%s replicating=%s readonly=%s lagged=%s",
			node.Name,
			nodeConditionStatus(node.Conditions, api.NodeConditionMaster),
			nodeConditionStatus(node.Conditions, api.NodeConditionReplicating),
			nodeConditionStatus(node.Conditions, api.NodeConditionReadOnly),
			nodeConditionStatus(node.Conditions, api.NodeConditionLagged),
		)
	}

	f.logClusterStatefulSet(cluster)
	f.logClusterPods(cluster)
	f.logClusterPVCs(cluster)
	f.logClusterEvents(cluster)
	Logf("=== end cluster readiness diagnostics: %s/%s ===", cluster.Namespace, cluster.Name)
}

func nodeConditionStatus(conds []api.NodeCondition, condType api.NodeConditionType) string {
	for _, cond := range conds {
		if cond.Type == condType {
			return string(cond.Status)
		}
	}
	return "Unknown"
}

func (f *Framework) logClusterStatefulSet(cluster *api.MysqlCluster) {
	stsName := GetNameForResource("sts", cluster)
	sts, err := f.ClientSet.AppsV1().StatefulSets(cluster.Namespace).Get(context.TODO(), stsName, metav1.GetOptions{})
	if err != nil {
		Logf("statefulset %s: get failed: %v", stsName, err)
		return
	}

	Logf("statefulset %s: replicas=%d ready=%d current=%d updated=%d observedGeneration=%d",
		stsName,
		int32Value(sts.Spec.Replicas),
		sts.Status.ReadyReplicas,
		sts.Status.CurrentReplicas,
		sts.Status.UpdatedReplicas,
		sts.Status.ObservedGeneration,
	)
}

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func (f *Framework) logClusterPods(cluster *api.MysqlCluster) {
	podList, err := f.ClientSet.CoreV1().Pods(cluster.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(GetClusterLabels(cluster)).String(),
	})
	if err != nil {
		Logf("cluster pods: list failed: %v", err)
		return
	}
	if len(podList.Items) == 0 {
		Logf("cluster pods: none found with labels %v", GetClusterLabels(cluster))
		return
	}

	for _, pod := range podList.Items {
		podReady := corev1.ConditionUnknown
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady {
				podReady = cond.Status
				break
			}
		}
		Logf("pod %s phase=%s ready=%s node=%s ip=%s",
			pod.Name, pod.Status.Phase, podReady, pod.Spec.NodeName, pod.Status.PodIP)
		for _, cs := range pod.Status.InitContainerStatuses {
			logContainerStatus("init", cs)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			logContainerStatus("main", cs)
		}
	}
}

func logContainerStatus(kind string, cs corev1.ContainerStatus) {
	msg := fmt.Sprintf("  %s container %q image=%q ready=%v restartCount=%d",
		kind, cs.Name, cs.Image, cs.Ready, cs.RestartCount)
	switch {
	case cs.State.Waiting != nil:
		msg += fmt.Sprintf(" state=Waiting reason=%q message=%q",
			cs.State.Waiting.Reason, cs.State.Waiting.Message)
	case cs.State.Running != nil:
		msg += " state=Running"
	case cs.State.Terminated != nil:
		msg += fmt.Sprintf(" state=Terminated reason=%q exitCode=%d message=%q",
			cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
	}
	Logf(msg)
}

func (f *Framework) logClusterPVCs(cluster *api.MysqlCluster) {
	pvcs := f.GetClusterPVCsFn(cluster)()
	if len(pvcs) == 0 {
		Logf("cluster PVCs: none found")
		return
	}
	for _, pvc := range pvcs {
		Logf("pvc %s phase=%s capacity=%s storageClass=%q",
			pvc.Name,
			pvc.Status.Phase,
			pvc.Status.Capacity.Storage().String(),
			stringValue(pvc.Spec.StorageClassName),
		)
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (f *Framework) logClusterEvents(cluster *api.MysqlCluster) {
	eventList, err := f.ClientSet.CoreV1().Events(cluster.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		Logf("cluster events: list failed: %v", err)
		return
	}

	stsName := GetNameForResource("sts", cluster)
	podPrefix := stsName + "-"
	pvcPrefix := "data-" + podPrefix

	var matches []corev1.Event
	for _, event := range eventList.Items {
		name := event.InvolvedObject.Name
		if name == stsName || strings.HasPrefix(name, podPrefix) || strings.HasPrefix(name, pvcPrefix) {
			matches = append(matches, event)
		}
	}
	if len(matches) == 0 {
		Logf("cluster events: none for statefulset/pods/pvcs of %s", stsName)
		return
	}

	sort.Slice(matches, func(i, j int) bool {
		return eventTime(matches[i]).After(eventTime(matches[j]))
	})
	if len(matches) > 30 {
		matches = matches[:30]
	}

	Logf("cluster events (latest %d):", len(matches))
	for _, event := range matches {
		Logf("  %s %s %s/%s: %s",
			eventTime(event).Format(time.RFC3339),
			event.Type,
			event.InvolvedObject.Kind,
			event.InvolvedObject.Name,
			strings.TrimSpace(event.Message),
		)
	}
}

func eventTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.CreationTimestamp.Time
}
