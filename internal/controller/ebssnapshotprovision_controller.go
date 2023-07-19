/*
Copyright 2023 @apanasiuk-el edenlabllc.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
)

// EBSSnapshotProvisionReconciler reconciles a EBSSnapshotProvision object
type EBSSnapshotProvisionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ebs.aws.edenlab.io,resources=ebssnapshotprovisions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EBSSnapshotProvision object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *EBSSnapshotProvisionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EBSSnapshotProvisionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ebsv1alpha1.EBSSnapshotProvision{}).
		Complete(r)
}
