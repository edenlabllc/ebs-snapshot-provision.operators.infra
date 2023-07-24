/*
Copyright 2023 @apanasiuk-el edenlabllc.
*/

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"ebs-snapshot-provision.operators.infra/internal/snapshot"

	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
)

// EBSSnapshotProvisionReconciler reconciles a EBSSnapshotProvision object
type EBSSnapshotProvisionReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	SnapshotCreator snapshot.Creator
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
	reqLogger := log.FromContext(ctx)
	ebsSnapshotProvision := &ebsv1alpha1.EBSSnapshotProvision{}
	count := 0

	if err := r.Client.Get(ctx, req.NamespacedName, ebsSnapshotProvision); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Error(nil, fmt.Sprintf("Can not find CRD by name: %s", req.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Create new snapshots definition
	reqLogger.Info(fmt.Sprintf("Get AWS snapshots for cluster name: %s", ebsSnapshotProvision.Spec.ClusterName))
	newVolumeSnapshots, newVolumeSnapshotContents, err := r.SnapshotCreator.CreateVolumeSnapshots(&snapshot.InputFromCRD{CRD: ebsSnapshotProvision})
	if err != nil {
		reqLogger.Error(err, fmt.Sprintf("Can not get AWS snapshots for cluster name: %s", ebsSnapshotProvision.Spec.ClusterName))
		ebsSnapshotProvision.Status.Phase = "Error"
		ebsSnapshotProvision.Status.Error = err.Error()
		if err := r.Status().Update(ctx, ebsSnapshotProvision); err != nil {
			reqLogger.Error(err, fmt.Sprintf("Unable to update status for CRD: %s", req.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Create volume snapshot & relevant volume snapshot content
	for key, snap := range newVolumeSnapshots {
		defVolumeSnapshot := &snapv1.VolumeSnapshot{}
		vsContent := newVolumeSnapshotContents[key].DeepCopy()
		// Create a new VolumeSnapshot
		if err = r.Client.Get(ctx, types.NamespacedName{Name: snap.Name, Namespace: snap.Namespace}, defVolumeSnapshot); err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info(fmt.Sprintf("Creating new volume snapshot content: %s", vsContent.Name))
				if result, err := r.createVSContent(ctx, vsContent, reqLogger); err != nil {
					ebsSnapshotProvision.Status.Phase = "Error Provisioning Volume Snapshot Content"
					ebsSnapshotProvision.Status.Error = err.Error()
					if err := r.Status().Update(ctx, ebsSnapshotProvision); err != nil {
						reqLogger.Error(err, fmt.Sprintf("Unable to update status for CRD: %s", req.Name))
						return ctrl.Result{}, nil
					}

					return result, err
				}

				reqLogger.Info(fmt.Sprintf("Creating new volume snapshot: %s for namespace: %s", snap.Name, snap.Namespace))
				if err = r.Client.Create(ctx, snap.DeepCopy()); err != nil {
					ebsSnapshotProvision.Status.Phase = "Error Provisioning Volume Snapshot"
					ebsSnapshotProvision.Status.Error = err.Error()
					if err := r.Status().Update(ctx, ebsSnapshotProvision); err != nil {
						reqLogger.Error(err, fmt.Sprintf("Unable to update status for CRD: %s", req.Name))
						return ctrl.Result{}, nil
					}

					return ctrl.Result{}, err
				}

				count++
			} else {
				return ctrl.Result{}, err
			}
		}
	}

	// ebsSnapshotProvision.Status.Conditions.
	ebsSnapshotProvision.Status.Phase = "Provisioned VolumeSnapshots"
	if count > 0 {
		ebsSnapshotProvision.Status.CreatedTime = &metav1.Time{Time: time.Now()}
		ebsSnapshotProvision.Status.Count = count
	}
	ebsSnapshotProvision.Status.Error = ""
	if err := r.Status().Update(ctx, ebsSnapshotProvision); err != nil {
		reqLogger.Error(err, fmt.Sprintf("Unable to update status for CRD: %s", req.Name))
		return ctrl.Result{}, nil
	} else {
		reqLogger.Info(fmt.Sprintf("Update status for CRD: %s", req.Name))
	}

	return ctrl.Result{RequeueAfter: ebsSnapshotProvision.Spec.Frequency.Duration}, nil
}

func (r *EBSSnapshotProvisionReconciler) createVSContent(ctx context.Context, vsContent *snapv1.VolumeSnapshotContent, reqLogger logr.Logger) (ctrl.Result, error) {
	if err := r.Client.Create(ctx, vsContent); err != nil {
		if errors.IsAlreadyExists(err) {
			reqLogger.Info(fmt.Sprintf("Volume snapshot content: %s was exists, will be recreated", vsContent.Name))
			if err = r.Client.Delete(ctx, vsContent); err != nil {
				return ctrl.Result{}, err
			}

			// Try to create an object volume snapshot content as soon as it becomes available after deletion
			for {
				if err = r.Client.Create(ctx, vsContent); err != nil {
					if errors.IsAlreadyExists(err) {
						continue
					}

					return ctrl.Result{}, err
				}

				return ctrl.Result{}, nil
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EBSSnapshotProvisionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ebsv1alpha1.EBSSnapshotProvision{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
