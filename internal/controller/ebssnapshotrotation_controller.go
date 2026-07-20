/*
Copyright 2026 Edenlab.

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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
	"ebs-snapshot-provision.operators.infra/internal/snapshot"
	"ebs-snapshot-provision.operators.infra/internal/status"
)

// EBSSnapshotRotationReconciler reconciles a EBSSnapshotRotation object
type EBSSnapshotRotationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotrotations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotrotations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotrotations/finalizers,verbs=update
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots;volumesnapshotcontents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (rr *EBSSnapshotRotationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logf.FromContext(ctx)
	statusMgr := status.New(rr.Client, rr.Scheme, reqLogger)
	snapshotMgr := snapshot.New(rr.Client, rr.Scheme, reqLogger, "Rotation", statusMgr)

	ebsSR := &ebsv1alpha1.EBSSnapshotRotation{}
	if err := rr.Client.Get(ctx, req.NamespacedName, ebsSR); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(nil, fmt.Sprintf("Can not find CRD by name: %s", req.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Nothing left to do only if every key currently in spec has actually
	// completed — not just "Phase was Completed at some point". This lets a
	// newly added key still run even after the resource previously finished.
	if allTargetsCompleted(ebsSR) {
		return ctrl.Result{}, nil
	}

	if err := statusMgr.SetRotationPhase(ctx, ebsSR, ebsv1alpha1.RotationPhasePending); err != nil {
		return ctrl.Result{}, err
	}

	if err := snapshotMgr.RotateSnapshots(ctx, ebsSR); err != nil {
		return ctrl.Result{}, err
	}

	if err := statusMgr.SetRotationPhase(ctx, ebsSR, ebsv1alpha1.RotationPhaseCompleted); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: frequencyOrDefault(ebsSR.Spec.Frequency)}, nil
}

// allTargetsCompleted reports whether every key currently in
// Spec.SnapshotRotations has a Completed status. Comparing against the
// current spec (not just the last-computed aggregate Phase) ensures a key
// added after the resource previously completed is still picked up.
func allTargetsCompleted(obj *ebsv1alpha1.EBSSnapshotRotation) bool {
	if len(obj.Spec.SnapshotRotations) == 0 {
		return false
	}

	for key := range obj.Spec.SnapshotRotations {
		ts, ok := obj.Status.Targets[key]
		if !ok || ts.Phase != ebsv1alpha1.RotationPhaseCompleted {
			return false
		}
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (rr *EBSSnapshotRotationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ebsv1alpha1.EBSSnapshotRotation{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("ebssnapshotrotation").
		Complete(rr)
}
