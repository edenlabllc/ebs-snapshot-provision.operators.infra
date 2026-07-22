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

// EBSSnapshotProvisionSpec defines the desired state of EBSSnapshotProvision
type EBSSnapshotProvisionSpec struct {
	// ClusterName is the name of the Kubernetes cluster whose EBS snapshots
	// should be provisioned. It is used to filter AWS snapshots by the
	// "kubernetes.io/cluster/<ClusterName>" tag.
	ClusterName string `json:"clusterName"`

	// Region is the AWS region to look up EBS snapshots in.
	Region string `json:"region"`

	// VolumeSnapshotClassName is the name of the VolumeSnapshotClass used
	// when creating VolumeSnapshot and VolumeSnapshotContent resources.
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`

	// Frequency defines how often the controller should reconcile and check
	// for new AWS snapshots to provision.
	Frequency *metav1.Duration `json:"frequency,omitempty"`

	// WaitForReadyToUse determines whether the controller should wait for
	// the created VolumeSnapshot to become readyToUse before finishing
	// provisioning. Defaults to false.
	WaitForReadyToUse bool `json:"waitForReadyToUse,omitempty"`
}

// EBSSnapshotProvisionStatus defines the observed state of EBSSnapshotProvision.
type EBSSnapshotProvisionStatus struct {
	// Phase reflects the high-level state of the last reconciliation.
	Phase ProvisionPhase `json:"phase,omitempty"`

	// Error contains the error message from the last failed reconciliation.
	// Empty when the last reconciliation succeeded.
	Error string `json:"error,omitempty"`

	// SnapshotsFound is the total number of AWS snapshots matched for this
	// cluster during the last reconciliation.
	SnapshotsFound int `json:"snapshotsFound,omitempty"`

	// SnapshotsCreatedByNamespace is the cumulative number of VolumeSnapshot
	// resources created per namespace across all reconciliations since this
	// resource was created.
	SnapshotsCreatedByNamespace map[string]int `json:"snapshotsCreatedByNamespace,omitempty"`

	// LastUpdated is the timestamp of the last status update.
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="SNAPSHOTS-FOUND",type=integer,JSONPath=`.status.snapshotsFound`
// +kubebuilder:printcolumn:name="LAST-UPDATED",type=date,JSONPath=`.status.lastUpdated`

// EBSSnapshotProvision is the Schema for the ebssnapshotprovisions API
type EBSSnapshotProvision struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of EBSSnapshotProvision
	// +required
	Spec EBSSnapshotProvisionSpec `json:"spec"`

	// status defines the observed state of EBSSnapshotProvision
	// +optional
	Status EBSSnapshotProvisionStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// EBSSnapshotProvisionList contains a list of EBSSnapshotProvision
type EBSSnapshotProvisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EBSSnapshotProvision `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EBSSnapshotProvision{}, &EBSSnapshotProvisionList{})
}

// GetStatus returns a pointer to this object's Status, for use by the
// internal/status package's generic Patch helper.
func (r *EBSSnapshotProvision) GetStatus() *EBSSnapshotProvisionStatus {
	return &r.Status
}

// SetLastUpdated implements status.StatusPtr for EBSSnapshotProvisionStatus.
func (s *EBSSnapshotProvisionStatus) SetLastUpdated(t *metav1.Time) {
	s.LastUpdated = t
}
