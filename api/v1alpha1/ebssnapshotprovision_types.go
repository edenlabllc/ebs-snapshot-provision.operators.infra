/*
Copyright 2023 @apanasiuk-el edenlabllc.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EBSSnapshotProvisionSpec defines the desired state of EBSSnapshotProvision
type EBSSnapshotProvisionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterName             string           `json:"clusterName"`
	Region                  string           `json:"region"`
	VolumeSnapshotClassName string           `json:"volumeSnapshotClassName"`
	Frequency               *metav1.Duration `json:"frequency"`
}

// EBSSnapshotProvisionStatus defines the observed state of EBSSnapshotProvision
type EBSSnapshotProvisionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	CreatedTime *metav1.Time `json:"createdTime,omitempty"`
	Error       string       `json:"error,omitempty"`
	Phase       string       `json:"phase,omitempty"`
	Count       int          `json:"count,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="CREATED-TIME",type=string,JSONPath=".status.createdTime"
//+kubebuilder:printcolumn:name="COUNT",type=integer,JSONPath=".status.count"

// EBSSnapshotProvision is the Schema for the ebssnapshotprovisions API
type EBSSnapshotProvision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EBSSnapshotProvisionSpec   `json:"spec,omitempty"`
	Status EBSSnapshotProvisionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EBSSnapshotProvisionList contains a list of EBSSnapshotProvision
type EBSSnapshotProvisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EBSSnapshotProvision `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EBSSnapshotProvision{}, &EBSSnapshotProvisionList{})
}
