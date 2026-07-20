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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EBSSnapshotRetention defines how many resulting snapshots to keep per PVC
// after rotation. Snapshots beyond MaxCount are deleted, oldest first.
type EBSSnapshotRetention struct {
	// MaxCount is the maximum number of VolumeSnapshot resources to keep per
	// PVC after this rotation runs. Snapshots exceeding this count are
	// deleted, oldest first, based on their creation timestamp.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MaxCount int `json:"maxCount"`

	// PurgePolicy controls how snapshots exceeding MaxCount are removed.
	// Defaults to Deferred.
	// +kubebuilder:default=Deferred
	PurgePolicy SnapshotPurgePolicy `json:"purgePolicy,omitempty"`
}

// SnapshotRotationTarget describes one independent rotation unit: a set of
// PVCs to snapshot and the retention policy to apply afterwards. Each key
// under EBSSnapshotRotationSpec.SnapshotRotations is executed and tracked
// independently — adding a new key to an existing resource runs only that
// new target, without re-running targets that already completed.
type SnapshotRotationTarget struct {
	// Namespace is the namespace to look up PersistentVolumeClaims in.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// PVCSelector selects which PersistentVolumeClaims in Namespace to
	// snapshot. If empty, all PVCs in Namespace are selected.
	PVCSelector *metav1.LabelSelector `json:"pvcSelector,omitempty"`

	// VolumeSnapshotClassName is the VolumeSnapshotClass used when creating
	// VolumeSnapshot resources.
	// +kubebuilder:validation:Required
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`

	// Retention defines how many snapshots to keep per PVC after this
	// target runs.
	// +kubebuilder:validation:Required
	Retention EBSSnapshotRetention `json:"retention"`
}

// EBSSnapshotRotationSpec defines the desired state of EBSSnapshotRotation.
// Each entry in SnapshotRotations is a one-shot rotation unit: on first
// reconcile of a given key, the controller creates a fresh VolumeSnapshot for
// every matched PersistentVolumeClaim, applies retention, and never
// re-executes that key afterwards (see Status.Targets[key].Phase).
type EBSSnapshotRotationSpec struct {
	// Frequency sets the safety-net resync interval after all targets complete.
	Frequency *metav1.Duration `json:"frequency,omitempty"`

	// SnapshotRotations maps an arbitrary target name to its rotation
	// configuration. Each key is executed and tracked independently.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	SnapshotRotations map[string]SnapshotRotationTarget `json:"snapshotRotations"`
}

// SnapshotRotationTargetStatus is the observed state of a single rotation
// target, keyed by the same name as in Spec.SnapshotRotations.
type SnapshotRotationTargetStatus struct {
	// Phase reflects whether this target's one-shot rotation has run yet,
	// and its outcome.
	Phase RotationPhase `json:"phase,omitempty"`

	// Error contains the error message if the rotation failed for this target.
	Error string `json:"error,omitempty"`

	// SnapshotsCreated lists the names of VolumeSnapshot resources created
	// for this target, one per matched PVC.
	SnapshotsCreated []string `json:"snapshotsCreated,omitempty"`

	// SnapshotsDeleted lists the names of VolumeSnapshot resources deleted
	// for this target to satisfy Retention.MaxCount.
	SnapshotsDeleted []string `json:"snapshotsDeleted,omitempty"`

	// CompletionTime is the timestamp when this target finished successfully.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// EBSSnapshotRotationStatus defines the observed state of EBSSnapshotRotation.
type EBSSnapshotRotationStatus struct {
	Phase RotationPhase `json:"phase,omitempty"`

	// Targets holds the observed state of each rotation target, keyed by the
	// same name as in Spec.SnapshotRotations.
	Targets map[string]SnapshotRotationTargetStatus `json:"targets,omitempty"`

	// LastUpdated is the timestamp of the last status update.
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="LAST-UPDATED",type=date,JSONPath=`.status.lastUpdated`

// EBSSnapshotRotation is the Schema for the ebssnapshotrotations API
type EBSSnapshotRotation struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of EBSSnapshotRotation
	// +required
	Spec EBSSnapshotRotationSpec `json:"spec"`

	// status defines the observed state of EBSSnapshotRotation
	// +optional
	Status EBSSnapshotRotationStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// EBSSnapshotRotationList contains a list of EBSSnapshotRotation
type EBSSnapshotRotationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EBSSnapshotRotation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EBSSnapshotRotation{}, &EBSSnapshotRotationList{})
}

// GetStatus returns a pointer to this object's Status, for use by the
// internal/status package's generic Patch helper.
func (r *EBSSnapshotRotation) GetStatus() *EBSSnapshotRotationStatus {
	return &r.Status
}

// SetLastUpdated implements status.StatusPtr for EBSSnapshotRotationStatus.
func (s *EBSSnapshotRotationStatus) SetLastUpdated(t *metav1.Time) {
	s.LastUpdated = t
}
