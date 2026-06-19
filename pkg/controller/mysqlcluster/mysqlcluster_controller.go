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

package mysqlcluster

import (
	"context"
	"reflect"
	"time"

	"github.com/presslabs/controller-util/syncer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mysqlv1alpha1 "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"

	api "github.com/codecapsules-io/mysql-operator/pkg/apis/mysql/v1alpha1"
	cleaner "github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/cleaner"
	clustersyncer "github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/syncer"
	"github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/upgrades"
	"github.com/codecapsules-io/mysql-operator/pkg/controller/mysqlcluster/internal/versionupgrade"
	"github.com/codecapsules-io/mysql-operator/pkg/internal/mysqlcluster"
	"github.com/codecapsules-io/mysql-operator/pkg/options"
)

var log = logf.Log.WithName(controllerName)

const controllerName = "controller.mysqlcluster"

// Add creates a new MysqlCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this mysql.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMysqlCluster{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor(controllerName),
		opt:      options.GetOptions(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MysqlCluster
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MysqlCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &policyv1.PodDisruptionBudget{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mysqlv1alpha1.MysqlCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMysqlCluster{}

// ReconcileMysqlCluster reconciles a MysqlCluster object
type ReconcileMysqlCluster struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	opt      *options.Options
}

// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets;services;events;jobs;pods;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=mysql.presslabs.org,resources=mysqlclusters;mysqlclusters/status;mysqlclusters/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a MysqlCluster object and makes changes based on the state read
// and what is in the MysqlCluster.Spec
// nolint: gocyclo
func (r *ReconcileMysqlCluster) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Fetch the MysqlCluster instance
	cluster := mysqlcluster.New(&mysqlv1alpha1.MysqlCluster{})
	err := r.Get(context.TODO(), request.NamespacedName, cluster.Unwrap())
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	// overwrite logger with cluster info
	// nolint: govet
	log := log.WithValues("key", request.NamespacedName)
	log.V(1).Info("reconcile cluster")

	// run upgrades
	// TODO: this should be removed in next version (v0.5)
	up := upgrades.NewUpgrader(r.Client, r.recorder, cluster, r.opt)
	if up.ShouldUpdate() {
		log.Info("the upgrader will run for this cluster")
		if err = up.Run(context.TODO()); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Update cluster spec that need to be saved
	spec := *cluster.Spec.DeepCopy()
	cluster.UpdateSpec()
	if !reflect.DeepEqual(spec, cluster.Spec) {
		sErr := r.Update(context.TODO(), cluster.Unwrap())
		if sErr != nil {
			log.Error(sErr, "failed to update cluster spec")
			return reconcile.Result{}, sErr
		}
		return reconcile.Result{}, nil
	}

	// Update FailoverInProgress condition to false when both Replicas and ReadyNodes are 0
	//
	// When a cluster's replica is set to 0, pods will be shutdown one by one,
	// during this process, orchestrator will try to do failover on this cluster
	// and set FailoverInProgress to True. Since all pods are deleted, the FailoverInProgress
	// condition will be true all the time. When user set cluster's replicas to a value greater than 0,
	// the pods of the cluster will boot one by one, but node controller will not init mysql when FailoverInProgress
	// is true.
	fip := cluster.GetClusterCondition(api.ClusterConditionFailoverInProgress)
	if fip != nil && fip.Status == corev1.ConditionTrue &&
		*cluster.Spec.Replicas == 0 && cluster.Status.ReadyNodes == 0 {

		cluster.UpdateStatusCondition(api.ClusterConditionFailoverInProgress, corev1.ConditionFalse,
			"ClusterNotRunning", "cluster is not running")

		if sErr := r.Status().Update(context.TODO(), cluster.Unwrap()); sErr != nil {
			log.Error(sErr, "failed to update cluster status")
			return reconcile.Result{}, sErr
		}
		return reconcile.Result{}, nil
	}

	// Set defaults on cluster
	r.scheme.Default(cluster.Unwrap())
	cluster.SetDefaults(r.opt)

	if err = cluster.Validate(); err != nil {
		return reconcile.Result{}, err
	}

	annBefore := cloneStringMap(cluster.Annotations)

	sts, stsErr := versionupgrade.GetStatefulSetForRollout(ctx, r.Client, cluster)
	if stsErr != nil {
		return reconcile.Result{}, stsErr
	}
	podList := &corev1.PodList{}
	if listErr := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(cluster.GetSelectorLabels())); listErr != nil {
		return reconcile.Result{}, listErr
	}
	if versionupgrade.NeedsAppliedBackfill(cluster) {
		updated, bfErr := versionupgrade.BackfillAppliedVersion(ctx, r.Client, cluster, podList.Items)
		if bfErr != nil {
			if versionupgrade.IsHoldRollout(bfErr) {
				log.Info("waiting for MySQL data-plane version backfill", "cluster", cluster, "reason", bfErr.Error())
				if syncErr := r.syncRolloutHold(ctx, cluster); syncErr != nil {
					return reconcile.Result{}, syncErr
				}
				if annErr := r.persistClusterAnnotations(ctx, cluster, annBefore); annErr != nil {
					return reconcile.Result{}, annErr
				}
				return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
			}
			return reconcile.Result{}, bfErr
		}
		if updated {
			if sErr := r.Status().Update(ctx, cluster.Unwrap()); sErr != nil {
				log.Error(sErr, "failed to persist applied MySQL version backfill")
				return reconcile.Result{}, sErr
			}
			if annErr := r.persistClusterAnnotations(ctx, cluster, annBefore); annErr != nil {
				return reconcile.Result{}, annErr
			}
			return reconcile.Result{Requeue: true}, nil
		}
	}

	if err = versionupgrade.EnsureChecked(ctx, r.Client, cluster); err != nil {
		if versionupgrade.IsHoldRollout(err) {
			log.Info("waiting for MySQL version upgrade", "cluster", cluster, "reason", err.Error())
			// Upgrade path is valid; clear a stale UpgradeBlocked from a prior invalid spec.
			if blocked := cluster.GetClusterCondition(api.ClusterConditionUpgradeBlocked); blocked != nil && blocked.Status == corev1.ConditionTrue {
				cluster.UpdateStatusCondition(api.ClusterConditionUpgradeBlocked, corev1.ConditionFalse,
					"NoUpgradePending", "")
				if sErr := r.Status().Update(ctx, cluster.Unwrap()); sErr != nil {
					log.Error(sErr, "failed to clear upgrade blocked condition")
					return reconcile.Result{}, sErr
				}
			}
			if syncErr := r.syncRolloutHold(ctx, cluster); syncErr != nil {
				return reconcile.Result{}, syncErr
			}
			if annErr := r.persistClusterAnnotations(ctx, cluster, annBefore); annErr != nil {
				return reconcile.Result{}, annErr
			}
			return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
		}
		log.Error(err, "MySQL version upgrade blocked", "cluster", cluster)
		r.recorder.Event(cluster.Unwrap(), corev1.EventTypeWarning, "MySQLVersionUpgradeBlocked", err.Error())
		// Use a dedicated condition so Ready is not corrupted and the cluster remains HA.
		cluster.UpdateStatusCondition(api.ClusterConditionUpgradeBlocked, corev1.ConditionTrue,
			"MySQLVersionUpgradeBlocked", err.Error())
		if sErr := r.Status().Update(ctx, cluster.Unwrap()); sErr != nil {
			log.Error(sErr, "failed to update cluster status")
			return reconcile.Result{}, sErr
		}
		if versionupgrade.IsUpgradeBlocked(err) {
			// Invalid upgrade path - cluster stays operational on current version.
			// syncRolloutHold pins the STS image/env at the current version via RolloutMySQLVersion.
			// Requeue slowly so that if the user corrects spec.mysqlVersion we re-validate promptly.
			if syncErr := r.syncRolloutHold(ctx, cluster); syncErr != nil {
				return reconcile.Result{}, syncErr
			}
			if annErr := r.persistClusterAnnotations(ctx, cluster, annBefore); annErr != nil {
				return reconcile.Result{}, annErr
			}
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		// For transient failures return the error to trigger backoff retry,
		// but do NOT set Ready=False - the cluster remains HA while we wait.
		return reconcile.Result{}, err
	}
	// Upgrade path is valid (or no upgrade pending) - clear any previous UpgradeBlocked condition.
	cluster.UpdateStatusCondition(api.ClusterConditionUpgradeBlocked, corev1.ConditionFalse,
		"NoUpgradePending", "")
	if annErr := r.persistClusterAnnotations(ctx, cluster, annBefore); annErr != nil {
		return reconcile.Result{}, annErr
	}

	status := *cluster.Status.DeepCopy()
	defer func() {
		if !reflect.DeepEqual(status, cluster.Status) {
			sErr := r.Status().Update(context.TODO(), cluster.Unwrap())
			if sErr != nil {
				log.Error(sErr, "failed to update cluster status")
			}
		}
	}()

	sts, stsErr = versionupgrade.GetStatefulSetForRollout(ctx, r.Client, cluster)
	if stsErr != nil {
		return reconcile.Result{}, stsErr
	}
	configMapSyncer := clustersyncer.NewConfigMapSyncer(r.Client, r.scheme, cluster, sts)
	if err = syncer.Sync(context.TODO(), configMapSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	secretSyncer := clustersyncer.NewOperatedSecretSyncer(r.Client, r.scheme, cluster, r.opt)
	if err = syncer.Sync(context.TODO(), secretSyncer, r.recorder); err != nil {
		return reconcile.Result{}, err
	}

	cmRev := configMapSyncer.Object().(*corev1.ConfigMap).ResourceVersion
	sctRev := secretSyncer.Object().(*corev1.Secret).ResourceVersion

	// run the syncers for services, pdb and statefulset
	syncers := []syncer.Interface{
		clustersyncer.NewSecretSyncer(r.Client, r.scheme, cluster, r.opt),
		clustersyncer.NewHeadlessSVCSyncer(r.Client, r.scheme, cluster),
		clustersyncer.NewMasterSVCSyncer(r.Client, r.scheme, cluster),
		clustersyncer.NewHealthySVCSyncer(r.Client, r.scheme, cluster),
		clustersyncer.NewHealthyReplicasSVCSyncer(r.Client, r.scheme, cluster),

		clustersyncer.NewStatefulSetSyncer(r.Client, r.scheme, cluster, cmRev, sctRev, r.opt),
	}

	if len(cluster.Spec.MinAvailable) != 0 {
		syncers = append(syncers, clustersyncer.NewPDBSyncer(r.Client, r.scheme, cluster))
	}

	for _, sync := range syncers {
		if err = syncer.Sync(context.TODO(), sync, r.recorder); err != nil {
			return reconcile.Result{}, err
		}
	}

	// run the pod syncers
	log.V(1).Info("cluster status", "status", cluster.Status)
	for _, sync := range r.getPodSyncers(cluster) {
		if err = syncer.Sync(context.TODO(), sync, r.recorder); err != nil {
			// if it's pod not found then skip the error
			if !clustersyncer.IsPodNotFound(err) {
				return reconcile.Result{}, err
			}
		}
	}

	// Expand existing data PVCs when volumeSpec storage increases (StatefulSet template alone does not resize claims).
	pvcResizer := cleaner.NewPVCResizer(cluster, r.opt, r.recorder, r.Client)
	if err = pvcResizer.Run(context.TODO()); err != nil {
		return reconcile.Result{}, err
	}

	// Perform any cleanup
	pvcCleaner := cleaner.NewPVCCleaner(cluster, r.opt, r.recorder, r.Client)
	err = pvcCleaner.Run(context.TODO())
	if err != nil {
		return reconcile.Result{}, err
	}

	if sts, stsErr := versionupgrade.GetStatefulSetForRollout(ctx, r.Client, cluster); stsErr != nil {
		return reconcile.Result{}, stsErr
	} else if sts != nil {
		podList := &corev1.PodList{}
		if listErr := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(cluster.GetSelectorLabels())); listErr != nil {
			return reconcile.Result{}, listErr
		}
		if advance, ok := versionupgrade.SyncAppliedVersion(ctx, r.Client, cluster, sts, podList.Items); ok {
			versionupgrade.MarkAppliedVersion(cluster, advance)
			if sErr := r.Status().Update(ctx, cluster.Unwrap()); sErr != nil {
				log.Error(sErr, "failed to persist applied MySQL version status")
				return reconcile.Result{}, sErr
			}
		}
	}

	return reconcile.Result{}, nil
}

// syncRolloutHold runs ConfigMap and StatefulSet sync so RolloutMySQLVersion can pin the
// data-plane image/env while the upgrade path is invalid.
func (r *ReconcileMysqlCluster) syncRolloutHold(ctx context.Context, cluster *mysqlcluster.MysqlCluster) error {
	sts, err := versionupgrade.GetStatefulSetForRollout(ctx, r.Client, cluster)
	if err != nil {
		return err
	}
	configMapSyncer := clustersyncer.NewConfigMapSyncer(r.Client, r.scheme, cluster, sts)
	if err = syncer.Sync(ctx, configMapSyncer, r.recorder); err != nil {
		return err
	}
	secretSyncer := clustersyncer.NewOperatedSecretSyncer(r.Client, r.scheme, cluster, r.opt)
	if err = syncer.Sync(ctx, secretSyncer, r.recorder); err != nil {
		return err
	}
	cmRev := configMapSyncer.Object().(*corev1.ConfigMap).ResourceVersion
	sctRev := secretSyncer.Object().(*corev1.Secret).ResourceVersion
	stsSyncer := clustersyncer.NewStatefulSetSyncer(r.Client, r.scheme, cluster, cmRev, sctRev, r.opt)
	return syncer.Sync(ctx, stsSyncer, r.recorder)
}

func (r *ReconcileMysqlCluster) persistClusterAnnotations(ctx context.Context, cluster *mysqlcluster.MysqlCluster, before map[string]string) error {
	if reflect.DeepEqual(before, cluster.Annotations) {
		return nil
	}
	return r.Update(ctx, cluster.Unwrap())
}

// cloneStringMap copies annotation maps; assigning a map only copies the reference in Go.
func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// getPodSyncers returns a list of syncers for every pod of the cluster. The
// list is sorted by node roles, first are replicas and the last is the master
// pod syncer. We need to have replicas first to avoid having two pods with
// master label in the same time. This can happen for a small period of time
// when master changes.
func (r *ReconcileMysqlCluster) getPodSyncers(cluster *mysqlcluster.MysqlCluster) []syncer.Interface {
	syncers := []syncer.Interface{}

	// add replica syncers, those should be the first in this list.
	for _, ns := range cluster.Status.Nodes {
		if !getCondAsBool(&ns, mysqlv1alpha1.NodeConditionMaster) {
			syncers = append(syncers, clustersyncer.NewPodSyncer(r.Client, r.scheme, cluster, ns.Name))
		}
	}

	// add master syncers, this should be the last, and should be only one
	for _, ns := range cluster.Status.Nodes {
		if getCondAsBool(&ns, mysqlv1alpha1.NodeConditionMaster) {
			syncers = append(syncers, clustersyncer.NewPodSyncer(r.Client, r.scheme, cluster, ns.Name))
		}
	}

	return syncers
}

func getCondAsBool(status *mysqlv1alpha1.NodeStatus, cond mysqlv1alpha1.NodeConditionType) bool {
	index, exists := mysqlcluster.GetNodeConditionIndex(status, cond)
	return exists && status.Conditions[index].Status == corev1.ConditionTrue
}
