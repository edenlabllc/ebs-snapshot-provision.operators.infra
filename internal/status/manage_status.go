package status

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
)

// ManageStatus updates CR status via the status subresource.
type ManageStatus struct {
	Client client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
}

// New returns a new status manager.
func New(c client.Client, r *runtime.Scheme, l logr.Logger) *ManageStatus {
	return &ManageStatus{Client: c, Scheme: r, Logger: l.WithName("Status")}
}

// Ptr is satisfied by *S for any Status struct managed by Patch. It
// must support SetLastUpdated — a one-line method added once per Status
// type, e.g.:
//
//	func (s *EBSSnapshotProvisionStatus) SetLastUpdated(t *metav1.Time) { s.LastUpdated = t }
type Ptr[S any] interface {
	*S
	DeepCopy() *S
	SetLastUpdated(*metav1.Time)
}

// Object is implemented by any CRD type managed by Patch: a
// client.Object exposing a pointer to its own Status field, e.g.:
//
//	func (r *EBSSnapshotProvision) GetStatus() *EBSSnapshotProvisionStatus { return &r.Status }
type Object[S any, PS Ptr[S]] interface {
	client.Object
	GetStatus() PS
}

// Patch mutates obj's status in place with mutate, then patches it via
// MergeFrom — skipping the API call entirely if nothing meaningful changed
// (LastUpdated is ignored during comparison). The call shape is identical
// for every CRD type: Patch(ctx, client, obj, mutate). Caller must pass a
// live object (fetched from the API).
func Patch[T Object[S, PS], S any, PS Ptr[S]](ctx context.Context, c client.Client, obj T, mutate func(PS)) error {
	oldObj, ok := obj.DeepCopyObject().(T)
	if !ok {
		return fmt.Errorf("status: unexpected DeepCopyObject() result type for %T", obj)
	}

	st := obj.GetStatus()
	before := *PS(st.DeepCopy())

	mutate(st)

	if cmp.Equal(before, *st, cmpopts.EquateEmpty(), cmpopts.IgnoreFields(before, "LastUpdated")) {
		return nil
	}

	now := metav1.NewTime(time.Now().UTC())
	st.SetLastUpdated(&now)

	return c.Status().Patch(ctx, obj, client.MergeFrom(oldObj))
}

// SetProvisionStatus updates the status of an EBSSnapshotProvision after a
// reconciliation attempt.
func (m *ManageStatus) SetProvisionStatus(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotProvision,
	phase ebsv1alpha1.ProvisionPhase, snapshotsFound int, createdByNamespace map[string]int, err error) error {
	return Patch(ctx, m.Client, obj, func(sp *ebsv1alpha1.EBSSnapshotProvisionStatus) {
		sp.Phase = phase
		sp.SnapshotsFound = snapshotsFound
		sp.SnapshotsCreatedByNamespace = createdByNamespace

		if err != nil {
			sp.Error = err.Error()
			return
		}

		sp.Error = ""
	})
}

// SetRotationTargetStatus updates the status of a single rotation target,
// keyed by targetKey, within EBSSnapshotRotation.Status.Targets.
func (m *ManageStatus) SetRotationTargetStatus(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotRotation,
	targetKey string, phase ebsv1alpha1.RotationPhase, created, deleted []string, err error) error {
	return Patch(ctx, m.Client, obj, func(sr *ebsv1alpha1.EBSSnapshotRotationStatus) {
		if sr.Targets == nil {
			sr.Targets = make(map[string]ebsv1alpha1.SnapshotRotationTargetStatus)
		}

		ts := sr.Targets[targetKey]
		ts.Phase = phase
		ts.SnapshotsCreated = created
		ts.SnapshotsDeleted = deleted

		if err != nil {
			ts.Error = err.Error()
		} else {
			ts.Error = ""
			now := metav1.NewTime(time.Now().UTC())
			ts.CompletionTime = &now
		}

		sr.Targets[targetKey] = ts
	})
}

// SetRotationPhase sets the aggregate Phase for the whole EBSSnapshotRotation
// resource, leaving Targets untouched.
func (m *ManageStatus) SetRotationPhase(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotRotation, phase ebsv1alpha1.RotationPhase) error {
	return Patch(ctx, m.Client, obj, func(sr *ebsv1alpha1.EBSSnapshotRotationStatus) {
		sr.Phase = phase
	})
}
