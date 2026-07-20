package snapshot

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
	"ebs-snapshot-provision.operators.infra/internal/aws"
	"ebs-snapshot-provision.operators.infra/internal/status"
)

const (
	labelSnapscheduler   = "snapscheduler.backube/when"
	snapshotTimeLayout   = "200601021504"
	vsDeletePollInterval = 5 * time.Second
	vsDeletePollTimeout  = 600 * time.Second
)

type ManageSnapshot struct {
	Client client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
	Status *status.ManageStatus
}

type Provisioner interface {
	ProvisionVolumeSnapshots(obj *ebsv1alpha1.EBSSnapshotProvision) ([]snapv1.VolumeSnapshot, []snapv1.VolumeSnapshotContent, error)
}

type DefaultSnapshotProvisioner struct {
	Retriever aws.SnapshotRetriever
}

var (
	volumeSnapshotMeta = metav1.TypeMeta{
		APIVersion: "snapshot.storage.k8s.io/v1",
		Kind:       "VolumeSnapshot",
	}

	volumeSnapshotContentMeta = metav1.TypeMeta{
		APIVersion: "snapshot.storage.k8s.io/v1",
		Kind:       "VolumeSnapshotContent",
	}

	volumeSnapshotContentSourceVolumeMode = v1.PersistentVolumeFilesystem
	snapshotTimeRe                        = regexp.MustCompile(`(\d{12})$`)
)

func NewDefaultSnapshotProvisioner(r aws.SnapshotRetriever) *DefaultSnapshotProvisioner {
	return &DefaultSnapshotProvisioner{
		Retriever: r,
	}
}

// extractTimeFromName pulls the trailing 12-digit timestamp out of a snapshot name.
func extractTimeFromName(name string) (string, error) {
	match := snapshotTimeRe.FindString(name)
	if match == "" {
		return "", fmt.Errorf("timestamp not found in snapshot name %q", name)
	}

	t, err := time.Parse(snapshotTimeLayout, match)
	if err != nil {
		return "", fmt.Errorf("failed to parse timestamp %q from snapshot name %q: %w", match, name, err)
	}

	return t.Format(snapshotTimeLayout), nil
}

// ProvisionVolumeSnapshots builds VolumeSnapshot/VolumeSnapshotContent objects
// (not yet applied) from AWS snapshots tagged for this cluster/region.
func (sp *DefaultSnapshotProvisioner) ProvisionVolumeSnapshots(obj *ebsv1alpha1.EBSSnapshotProvision) ([]snapv1.VolumeSnapshot, []snapv1.VolumeSnapshotContent, error) {
	var (
		volumeSnapshots        []snapv1.VolumeSnapshot
		volumeSnapshotContents []snapv1.VolumeSnapshotContent
	)

	snapshots, err := sp.Retriever.GetAWSSnapshots(obj.Spec.ClusterName, obj.Spec.Region)
	if err != nil {
		return nil, nil, err
	}

	for _, snapshot := range snapshots {
		var (
			namespace                 string
			name                      string
			volumeSnapshotContentName string
		)

		for _, tag := range snapshot.Tags {
			if *tag.Key == "snapshotNamespace" {
				namespace = *tag.Value
			}

			if *tag.Key == "snapshotName" {
				name = *tag.Value
			}

			if *tag.Key == "snapshotContentName" {
				volumeSnapshotContentName = *tag.Value
			}

		}

		if namespace == "" || name == "" || volumeSnapshotContentName == "" {
			return nil, nil, fmt.Errorf(
				"snapshot %s is missing required tags (namespace=%q, name=%q, contentName=%v)",
				*snapshot.SnapshotId, namespace, name, volumeSnapshotContentName,
			)
		}

		snapshotTime, err := extractTimeFromName(name)
		if err != nil {
			return nil, nil, err
		}

		snapshotLabels := map[string]string{labelSnapscheduler: snapshotTime}

		volumeSnapshots = append(volumeSnapshots,
			buildVolumeSnapshot(name, namespace, snapshotLabels, obj.Spec.VolumeSnapshotClassName,
				snapv1.VolumeSnapshotSource{VolumeSnapshotContentName: &volumeSnapshotContentName}))

		volumeSnapshotContents = append(volumeSnapshotContents, snapv1.VolumeSnapshotContent{
			TypeMeta: volumeSnapshotContentMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name: volumeSnapshotContentName,
			},
			Spec: snapv1.VolumeSnapshotContentSpec{
				VolumeSnapshotRef: v1.ObjectReference{
					Kind:       volumeSnapshotMeta.Kind,
					Namespace:  namespace,
					Name:       name,
					APIVersion: volumeSnapshotMeta.APIVersion,
				},
				DeletionPolicy:          snapv1.VolumeSnapshotContentRetain,
				Driver:                  "ebs.csi.aws.com",
				VolumeSnapshotClassName: &obj.Spec.VolumeSnapshotClassName,
				Source: snapv1.VolumeSnapshotContentSource{
					SnapshotHandle: snapshot.SnapshotId,
				},
				SourceVolumeMode: &volumeSnapshotContentSourceVolumeMode,
			},
			Status: nil,
		})
	}

	return volumeSnapshots, volumeSnapshotContents, nil
}

func New(c client.Client, s *runtime.Scheme, l logr.Logger, loggerName string, status *status.ManageStatus) *ManageSnapshot {
	return &ManageSnapshot{Client: c, Scheme: s, Logger: l.WithName(loggerName), Status: status}
}

// buildVolumeSnapshot constructs a VolumeSnapshot object; caller must Create it.
func buildVolumeSnapshot(name, namespace string, labels map[string]string, volumeSnapshotClassName string, source snapv1.VolumeSnapshotSource) snapv1.VolumeSnapshot {
	className := volumeSnapshotClassName // local copy, safe to take its address

	return snapv1.VolumeSnapshot{
		TypeMeta: volumeSnapshotMeta,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    labels,
		},
		Spec: snapv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &className,
			Source:                  source,
		},
		Status: nil,
	}
}

// ReproduceVolumeSnapshots creates any missing VolumeSnapshot/VolumeSnapshotContent
// pairs in the cluster and updates EBSSnapshotProvision status accordingly.
func (ms *ManageSnapshot) ReproduceVolumeSnapshots(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotProvision, volumeSnapshots []snapv1.VolumeSnapshot, volumeSnapshotContents []snapv1.VolumeSnapshotContent) error {
	createdByNamespace := make(map[string]int, len(obj.Status.SnapshotsCreatedByNamespace))
	for ns, n := range obj.Status.SnapshotsCreatedByNamespace {
		createdByNamespace[ns] = n
	}

	var provisioningSet bool

	for key, snap := range volumeSnapshots {
		newVS := &snapv1.VolumeSnapshot{}
		vsContent := volumeSnapshotContents[key].DeepCopy()

		if err := ms.ensureNamespace(ctx, snap.Namespace); err != nil {
			ms.Logger.Error(err, fmt.Sprintf("Unable to ensure namespace %s exists", snap.Namespace))
			if stErr := ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseFailed, len(volumeSnapshots), createdByNamespace, err); stErr != nil {
				return stErr
			}

			return err
		}

		if err := ms.Client.Get(ctx, types.NamespacedName{Name: snap.Name, Namespace: snap.Namespace}, newVS); err != nil {
			if errors.IsNotFound(err) {
				if !provisioningSet {
					if stErr := ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseProvisioning, len(volumeSnapshots), createdByNamespace, nil); stErr != nil {
						return stErr
					}

					provisioningSet = true
				}

				if err := ms.createVSContent(ctx, vsContent); err != nil {
					if stErr := ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseFailed, len(volumeSnapshots), createdByNamespace, err); stErr != nil {
						return stErr
					}

					return err
				}

				ms.Logger.Info(fmt.Sprintf("Creating new volume snapshot: %s for namespace: %s", snap.Name, snap.Namespace))
				if err = ms.Client.Create(ctx, snap.DeepCopy()); err != nil {
					if stErr := ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseFailed, len(volumeSnapshots), createdByNamespace, err); stErr != nil {
						return stErr
					}

					return err
				}

				createdByNamespace[snap.Namespace]++

				if obj.Spec.WaitForReadyToUse {
					if err := ms.waitForVSReadyToUse(ctx, snap.Name, snap.Namespace); err != nil {
						if stErr := ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseFailed, len(volumeSnapshots), createdByNamespace, err); stErr != nil {
							return stErr
						}

						return err
					}
				}
			} else {
				return err
			}
		}
	}

	if obj.Status.Phase != ebsv1alpha1.PhaseProvisioned {
		return ms.Status.SetProvisionStatus(ctx, obj, ebsv1alpha1.PhaseProvisioned, len(volumeSnapshots), createdByNamespace, nil)
	}

	return nil
}

// waitForVSReadyToUse polls until the VolumeSnapshot reports readyToUse=true,
// or returns an error if the snapshot itself reports a failure.
func (ms *ManageSnapshot) waitForVSReadyToUse(ctx context.Context, name, namespace string) error {
	vs := &snapv1.VolumeSnapshot{}

	return wait.PollUntilContextTimeout(ctx, vsDeletePollInterval, vsDeletePollTimeout, true,
		func(ctx context.Context) (bool, error) {
			if err := ms.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, vs); err != nil {
				if errors.IsNotFound(err) {
					return false, nil
				}

				return false, err
			}

			if vs.Status != nil && vs.Status.Error != nil {
				return false, fmt.Errorf("volume snapshot %s/%s failed: %s", namespace, name, *vs.Status.Error.Message)
			}

			if vs.Status != nil && vs.Status.ReadyToUse != nil && *vs.Status.ReadyToUse {
				return true, nil
			}

			return false, nil
		},
	)
}

// createVSContent creates a VolumeSnapshotContent, recreating it (delete + wait + create) if it already exists.
func (ms *ManageSnapshot) createVSContent(ctx context.Context, vsContent *snapv1.VolumeSnapshotContent) error {
	ms.Logger.Info(fmt.Sprintf("Creating new volume snapshot content: %s", vsContent.Name))

	if err := ms.Client.Create(ctx, vsContent); err == nil {
		return nil
	} else if !errors.IsAlreadyExists(err) {
		return fmt.Errorf("creating volume snapshot content %s: %w", vsContent.Name, err)
	}

	ms.Logger.Info(fmt.Sprintf("Volume snapshot content %s already exists, recreating", vsContent.Name))

	if err := ms.Client.Delete(ctx, vsContent); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting volume snapshot content %s: %w", vsContent.Name, err)
	}

	if err := ms.waitForVSContentDeleted(ctx, vsContent.Name); err != nil {
		return fmt.Errorf("waiting for volume snapshot content %s to be deleted: %w", vsContent.Name, err)
	}

	if err := ms.Client.Create(ctx, vsContent); err != nil {
		return fmt.Errorf("recreating volume snapshot content %s: %w", vsContent.Name, err)
	}

	return nil
}

// waitForVSContentDeleted polls until the named VolumeSnapshotContent no longer exists.
func (ms *ManageSnapshot) waitForVSContentDeleted(ctx context.Context, name string) error {
	existing := &snapv1.VolumeSnapshotContent{}

	return wait.PollUntilContextTimeout(ctx, vsDeletePollInterval, vsDeletePollTimeout, true,
		func(ctx context.Context) (bool, error) {
			if err := ms.Client.Get(ctx, types.NamespacedName{Name: name}, existing); err != nil {
				if errors.IsNotFound(err) {
					return true, nil
				}

				return false, err
			}

			return false, nil
		},
	)
}

// ensureNamespace creates the given namespace if it doesn't already exist.
func (ms *ManageSnapshot) ensureNamespace(ctx context.Context, namespace string) error {
	ns := &v1.Namespace{}
	if err := ms.Client.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			ms.Logger.Info(fmt.Sprintf("Namespace %s not found, creating it", namespace))
			newNs := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			if createErr := ms.Client.Create(ctx, newNs); createErr != nil {
				if errors.IsAlreadyExists(createErr) {
					return nil
				}

				return fmt.Errorf("failed to create namespace %s: %w", namespace, createErr)
			}

			return nil
		}

		return fmt.Errorf("failed to get namespace %s: %w", namespace, err)
	}

	return nil
}

// RotateSnapshots runs every not-yet-completed target in obj.Spec.SnapshotRotations.
// Each target is one-shot: once Completed, it's skipped forever (until the CR is deleted).
func (ms *ManageSnapshot) RotateSnapshots(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotRotation) error {
	for targetKey, target := range obj.Spec.SnapshotRotations {
		if existing, ok := obj.Status.Targets[targetKey]; ok && existing.Phase == ebsv1alpha1.RotationPhaseCompleted {
			continue
		}

		ms.Logger.Info(fmt.Sprintf("Target %s: starting rotation", targetKey))

		created, deleted, err := ms.runTarget(ctx, obj, targetKey, target)
		if err != nil {
			if stErr := ms.Status.SetRotationTargetStatus(ctx, obj, targetKey, ebsv1alpha1.RotationPhaseFailed, created, deleted, err); stErr != nil {
				return stErr
			}
			return err
		}

		if err := ms.Status.SetRotationTargetStatus(ctx, obj, targetKey, ebsv1alpha1.RotationPhaseCompleted, created, deleted, nil); err != nil {
			return err
		}

		ms.Logger.Info(fmt.Sprintf("Target %s: rotation completed", targetKey))
	}

	return nil
}

type pvcWork struct {
	name      string
	prefix    string
	succeeded bool
}

// runTarget snapshots every PVC matched by target.PVCSelector concurrently
// under one shared timestamp, then waits for readiness and retains per PVC.
func (ms *ManageSnapshot) runTarget(ctx context.Context, obj *ebsv1alpha1.EBSSnapshotRotation, targetKey string, target ebsv1alpha1.SnapshotRotationTarget) ([]string, []string, error) {
	selector := labels.Everything()
	if target.PVCSelector != nil {
		s, err := metav1.LabelSelectorAsSelector(target.PVCSelector)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid pvcSelector for target %q: %w", targetKey, err)
		}
		selector = s
	}

	pvcList := &v1.PersistentVolumeClaimList{}
	if err := ms.Client.List(ctx, pvcList, client.InNamespace(target.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, nil, fmt.Errorf("listing PVCs in namespace %q: %w", target.Namespace, err)
	}

	ms.Logger.Info(fmt.Sprintf("Target %s: matched %d PVCs in namespace %s", targetKey, len(pvcList.Items), target.Namespace))

	alreadyCreated := existingCreatedFor(obj, targetKey)
	when := time.Now().UTC().Format(snapshotTimeLayout)
	work := make([]pvcWork, len(pvcList.Items))
	g, gctx := errgroup.WithContext(ctx)

	for i := range pvcList.Items {
		pvcName := pvcList.Items[i].Name
		prefix := fmt.Sprintf("%s-%s-", pvcName, targetKey)

		existingName := ""
		for _, n := range alreadyCreated {
			if strings.HasPrefix(n, prefix) {
				existingName = n
				break
			}
		}

		if existingName != "" {
			ms.Logger.Info(fmt.Sprintf("Target %s: reusing already-created snapshot %s for PVC %s", targetKey, existingName, pvcName))
			work[i] = pvcWork{name: existingName, prefix: prefix, succeeded: true}
			continue
		}

		name := prefix + when
		work[i] = pvcWork{name: name, prefix: prefix}

		g.Go(func() error {
			ms.Logger.Info(fmt.Sprintf("Target %s: creating volume snapshot %s for PVC %s", targetKey, name, pvcName))

			vs := buildVolumeSnapshot(
				name,
				target.Namespace,
				map[string]string{labelSnapscheduler: when},
				target.VolumeSnapshotClassName,
				snapv1.VolumeSnapshotSource{PersistentVolumeClaimName: &pvcName},
			)
			if err := ms.Client.Create(gctx, &vs); err != nil {
				return fmt.Errorf("creating volume snapshot %s: %w", name, err)
			}

			ms.Logger.Info(fmt.Sprintf("Target %s: created volume snapshot %s", targetKey, name))
			work[i].succeeded = true
			return nil
		})
	}

	createErr := g.Wait()

	var allCreated []string
	for _, w := range work {
		if w.succeeded {
			allCreated = append(allCreated, w.name)
		}
	}

	if createErr != nil {
		ms.Logger.Info(fmt.Sprintf("Target %s: snapshot creation failed: %s", targetKey, createErr))
		return allCreated, nil, createErr
	}

	ms.Logger.Info(fmt.Sprintf("Target %s: all %d snapshots created, applying retention", targetKey, len(allCreated)))

	var allDeleted []string
	for _, w := range work {
		ms.Logger.Info(fmt.Sprintf("Target %s: waiting for %s to become ready", targetKey, w.name))

		if err := ms.waitForVSReadyToUse(ctx, w.name, target.Namespace); err != nil {
			return allCreated, allDeleted, fmt.Errorf("waiting for volume snapshot %s to become ready: %w", w.name, err)
		}

		ms.Logger.Info(fmt.Sprintf("Target %s: %s is ready, applying retention for prefix %s", targetKey, w.name, w.prefix))

		deleted, err := ms.retainByPrefix(ctx, target.Namespace, w.prefix, target.Retention)
		allDeleted = append(allDeleted, deleted...)
		if err != nil {
			return allCreated, allDeleted, fmt.Errorf("applying retention for prefix %q: %w", w.prefix, err)
		}

		if len(deleted) > 0 {
			ms.Logger.Info(fmt.Sprintf("Target %s: removed %d snapshot(s) for prefix %s: %v", targetKey, len(deleted), w.prefix, deleted))
		}
	}

	ms.Logger.Info(fmt.Sprintf("Target %s: completed, created=%d deleted=%d", targetKey, len(allCreated), len(allDeleted)))

	return allCreated, allDeleted, nil
}

// existingCreatedFor returns snapshot names already recorded for targetKey from a previous, failed attempt.
func existingCreatedFor(obj *ebsv1alpha1.EBSSnapshotRotation, targetKey string) []string {
	if obj.Status.Targets == nil {
		return nil
	}

	return obj.Status.Targets[targetKey].SnapshotsCreated
}

// retainByPrefix keeps the newest retention.MaxCount VolumeSnapshots whose
// name starts with prefix, removing the rest (name sort == chronological
// order, since the timestamp suffix is fixed-width).
func (ms *ManageSnapshot) retainByPrefix(ctx context.Context, namespace, prefix string, retention ebsv1alpha1.EBSSnapshotRetention) ([]string, error) {
	list := &snapv1.VolumeSnapshotList{}
	if err := ms.Client.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("listing volume snapshots in namespace %q: %w", namespace, err)
	}

	var matched []snapv1.VolumeSnapshot
	for _, vs := range list.Items {
		if strings.HasPrefix(vs.Name, prefix) {
			matched = append(matched, vs)
		}
	}

	if len(matched) <= retention.MaxCount {
		return nil, nil
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})

	toDelete := matched[:len(matched)-retention.MaxCount]

	var deletedNames []string
	for i := range toDelete {
		vs := &toDelete[i]
		if err := ms.deleteSnapshot(ctx, vs, retention.PurgePolicy); err != nil {
			return deletedNames, fmt.Errorf("removing volume snapshot %s: %w", vs.Name, err)
		}
		deletedNames = append(deletedNames, vs.Name)
	}

	return deletedNames, nil
}

// deleteSnapshot flips the bound VolumeSnapshotContent to DeletionPolicy:
// Delete, then removes the VolumeSnapshot immediately if purgePolicy is
// Immediate, or leaves it for later (namespace/cluster teardown) if Deferred.
func (ms *ManageSnapshot) deleteSnapshot(ctx context.Context, vs *snapv1.VolumeSnapshot, purgePolicy ebsv1alpha1.SnapshotPurgePolicy) error {
	if vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil {
		contentName := *vs.Status.BoundVolumeSnapshotContentName
		vsc := &snapv1.VolumeSnapshotContent{}

		if err := ms.Client.Get(ctx, types.NamespacedName{Name: contentName}, vsc); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("getting volume snapshot content %s: %w", contentName, err)
			}
		} else if vsc.Spec.DeletionPolicy != snapv1.VolumeSnapshotContentDelete {
			patch := client.MergeFrom(vsc.DeepCopy())
			vsc.Spec.DeletionPolicy = snapv1.VolumeSnapshotContentDelete
			if err := ms.Client.Patch(ctx, vsc, patch); err != nil {
				return fmt.Errorf("patching deletion policy for volume snapshot content %s: %w", contentName, err)
			}
		}
	}

	if purgePolicy != ebsv1alpha1.PurgePolicyImmediate {
		return nil
	}

	if err := ms.Client.Delete(ctx, vs); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting volume snapshot %s: %w", vs.Name, err)
	}

	return nil
}
