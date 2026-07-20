package v1alpha1

// RotationPhase represents the high-level state of a single rotation target.
// +kubebuilder:validation:Enum=Pending;Completed;Failed
type RotationPhase string

const (
	// RotationPhasePending means this target has not been executed yet.
	RotationPhasePending RotationPhase = "Pending"

	// RotationPhaseCompleted means this target ran successfully exactly once
	// and will not be re-executed.
	RotationPhaseCompleted RotationPhase = "Completed"

	// RotationPhaseFailed means the rotation attempt for this target failed.
	// The controller will retry it on the next reconcile, since the phase
	// never reached Completed.
	RotationPhaseFailed RotationPhase = "Failed"
)

// SnapshotPurgePolicy controls how VolumeSnapshots exceeding
// Retention.MaxCount are removed.
// +kubebuilder:validation:Enum=Deferred;Immediate
type SnapshotPurgePolicy string

const (
	// PurgePolicyDeferred flips the bound VolumeSnapshotContent's
	// DeletionPolicy from Retain to Delete, but leaves the VolumeSnapshot
	// object in place. Actual removal happens later, whenever its namespace
	// is deleted (e.g. cluster teardown) — Kubernetes then garbage collects
	// the VolumeSnapshot/VolumeSnapshotContent, and the already-flipped
	// DeletionPolicy ensures the underlying EBS snapshot is removed too.
	PurgePolicyDeferred SnapshotPurgePolicy = "Deferred"

	// PurgePolicyImmediate flips the DeletionPolicy the same way, then
	// deletes the VolumeSnapshot object right away, cascading immediate
	// removal of the VolumeSnapshotContent and the underlying EBS snapshot.
	PurgePolicyImmediate SnapshotPurgePolicy = "Immediate"
)
