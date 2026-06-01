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
package mysqlcluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-test/deep"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/presslabs/controller-util/syncer"
)

// statefulSetObjectSyncer persists StatefulSets with CreateOrUpdate and conflict retry.
// Strategic merge patch can fail to replace pod template initContainers reliably; a full spec update is required
// when adding mysql-datadir-chown to an existing StatefulSet.
type statefulSetObjectSyncer struct {
	Owner          client.Object
	Obj            *apps.StatefulSet
	SyncFn         func(ctx context.Context) error
	Name           string
	Client         client.Client
	previousObject runtime.Object
}

func (s *statefulSetObjectSyncer) objectType(obj runtime.Object) string {
	if obj != nil {
		gvk, err := apiutil.GVKForObject(obj, s.Client.Scheme())
		if err != nil {
			return fmt.Sprintf("%T", obj)
		}
		return gvk.String()
	}
	return "nil"
}

func (s *statefulSetObjectSyncer) Object() interface{} {
	return s.Obj
}

func (s *statefulSetObjectSyncer) GetObject() interface{} {
	return s.Object()
}

func (s *statefulSetObjectSyncer) ObjectOwner() runtime.Object {
	return s.Owner
}

func (s *statefulSetObjectSyncer) GetOwner() runtime.Object {
	return s.ObjectOwner()
}

func (s *statefulSetObjectSyncer) Sync(ctx context.Context) (syncer.SyncResult, error) {
	result := syncer.SyncResult{}
	log := logf.FromContext(ctx, "syncer", s.Name)
	key := client.ObjectKeyFromObject(s.Obj)

	var op controllerutil.OperationResult
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var syncErr error
		op, syncErr = controllerutil.CreateOrUpdate(ctx, s.Client, s.Obj, s.mutateFn(ctx))
		return syncErr
	})
	result.Operation = op

	diff := deep.Equal(s.previousObject, s.Obj)

	if errors.Is(err, syncer.ErrOwnerDeleted) {
		log.Info(string(result.Operation), "key", key, "kind", s.objectType(s.Obj), "error", err)
		err = nil
	} else if errors.Is(err, syncer.ErrIgnore) {
		log.V(1).Info("syncer skipped", "key", key, "kind", s.objectType(s.Obj), "error", err)
		err = nil
	} else if err != nil {
		result.SetEventData("Warning", statefulSetEventReason(s.Name, err),
			fmt.Sprintf("%s %s failed syncing: %s", s.objectType(s.Obj), key, err))
		log.Error(err, string(result.Operation), "key", key, "kind", s.objectType(s.Obj), "diff", diff)
	} else {
		result.SetEventData("Normal", statefulSetEventReason(s.Name, err),
			fmt.Sprintf("%s %s %s successfully", s.objectType(s.Obj), key, result.Operation))
		log.V(1).Info(string(result.Operation), "key", key, "kind", s.objectType(s.Obj), "diff", diff)
	}

	return result, err
}

func (s *statefulSetObjectSyncer) mutateFn(ctx context.Context) controllerutil.MutateFn {
	return func() error {
		s.previousObject = s.Obj.DeepCopyObject()

		if err := s.SyncFn(ctx); err != nil {
			return err
		}

		if s.Owner == nil {
			return nil
		}

		if s.Owner.GetDeletionTimestamp().IsZero() {
			if err := controllerutil.SetControllerReference(s.Owner, s.Obj, s.Client.Scheme()); err != nil {
				return err
			}
		} else if ctime := s.Obj.GetCreationTimestamp(); ctime.IsZero() {
			return syncer.ErrOwnerDeleted
		}

		return nil
	}
}

func statefulSetEventReason(objKindName string, err error) string {
	if err != nil {
		return fmt.Sprintf("%sSyncFailed", objKindName)
	}
	return fmt.Sprintf("%sSyncSuccessfull", objKindName)
}

func newStatefulSetObjectSyncer(
	name string,
	owner client.Object,
	obj *apps.StatefulSet,
	c client.Client,
	syncFn func(ctx context.Context) error,
) syncer.Interface {
	return &statefulSetObjectSyncer{
		Owner:  owner,
		Obj:    obj,
		SyncFn: syncFn,
		Name:   name,
		Client: c,
	}
}
