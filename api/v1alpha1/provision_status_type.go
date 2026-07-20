package v1alpha1

// ProvisionPhase represents the current phase of an EBSSnapshotProvision reconciliation.
type ProvisionPhase string

const (
	// PhaseFailed means the last reconciliation encountered an error.
	PhaseFailed ProvisionPhase = "Failed"

	// PhaseProvisioned means all matched AWS snapshots have been reproduced
	// as VolumeSnapshot resources.
	PhaseProvisioned ProvisionPhase = "Provisioned"

	// PhaseProvisioning means VolumeSnapshot creation is in progress.
	PhaseProvisioning ProvisionPhase = "Provisioning"
)
