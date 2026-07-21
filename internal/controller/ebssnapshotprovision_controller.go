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

// EBSSnapshotProvisionReconciler reconciles a EBSSnapshotProvision object
type EBSSnapshotProvisionReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	SnapshotProvisioner snapshot.Provisioner
}

// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots;volumesnapshotcontents,verbs=get;list;watch;create;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (pr *EBSSnapshotProvisionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := logf.FromContext(ctx)
	statusMgr := status.New(pr.Client, pr.Scheme, reqLogger)
	snapshotMgr := snapshot.New(pr.Client, pr.Scheme, reqLogger, "Provision", statusMgr)

	ebsSP := &ebsv1alpha1.EBSSnapshotProvision{}
	if err := pr.Client.Get(ctx, req.NamespacedName, ebsSP); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(nil, fmt.Sprintf("Can not find CRD by name: %s", req.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Create new snapshots definition
	reqLogger.WithName("Provision").Info(fmt.Sprintf("Get AWS snapshots for cluster name: %s", ebsSP.Spec.ClusterName))
	newVolumeSnapshots, newVolumeSnapshotContents, err := pr.SnapshotProvisioner.ProvisionVolumeSnapshots(ebsSP)
	if err != nil {
		reqLogger.Error(err, fmt.Sprintf("Can not get AWS snapshots for cluster name: %s", ebsSP.Spec.ClusterName))
		if stErr := statusMgr.SetProvisionStatus(ctx, ebsSP, ebsv1alpha1.PhaseFailed, 0, nil, err); stErr != nil {
			return ctrl.Result{}, stErr
		}

		return ctrl.Result{}, err
	}

	if err := snapshotMgr.ReproduceVolumeSnapshots(ctx, ebsSP, newVolumeSnapshots, newVolumeSnapshotContents); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: frequencyOrDefault(ebsSP.Spec.Frequency)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (pr *EBSSnapshotProvisionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ebsv1alpha1.EBSSnapshotProvision{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("ebssnapshotprovision").
		Complete(pr)
}
