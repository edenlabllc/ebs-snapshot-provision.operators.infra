package snapshot

import (
	ebsv1alpha1 "ebs-snapshot-provision.operators.infra/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"ebs-snapshot-provision.operators.infra/internal/aws"
)

var (
	volumeSnapshotMeta = metav1.TypeMeta{
		APIVersion: "snapshot.storage.k8s.io/v1",
		Kind:       "VolumeSnapshot",
	}

	volumeSnapshotContentMeta = metav1.TypeMeta{
		APIVersion: "snapshot.storage.k8s.io/v1",
		Kind:       "VolumeSnapshotContent",
	}

	volumeSnapshotContentSourceVolumeMode = core_v1.PersistentVolumeFilesystem
)

type InputFromCRD struct {
	CRD *ebsv1alpha1.EBSSnapshotProvision
}

type Creator interface {
	CreateVolumeSnapshots(input *InputFromCRD) ([]snapv1.VolumeSnapshot, []snapv1.VolumeSnapshotContent, error)
}

type DefaultSnapshotCreator struct {
	r aws.SnapshotRetriever
}

func NewDefaultSnapshotCreator(r aws.SnapshotRetriever) *DefaultSnapshotCreator {
	return &DefaultSnapshotCreator{
		r: r,
	}
}

func (sg *DefaultSnapshotCreator) CreateVolumeSnapshots(input *InputFromCRD) ([]snapv1.VolumeSnapshot, []snapv1.VolumeSnapshotContent, error) {
	var (
		volumeSnapshots           []snapv1.VolumeSnapshot
		volumeSnapshotContents    []snapv1.VolumeSnapshotContent
		namespace                 string
		name                      string
		volumeSnapshotContentName *string
	)

	snapshots, err := sg.r.GetSnapshots(input.CRD.Spec.ClusterName, input.CRD.Spec.Region)
	if err != nil {
		return nil, nil, err
	}

	for _, snapshot := range snapshots {
		for _, tag := range snapshot.Tags {
			if *tag.Key == "snapshotNamespace" {
				namespace = *tag.Value
			}

			if *tag.Key == "snapshotName" {
				name = *tag.Value
			}

			if *tag.Key == "snapshotContentName" {
				volumeSnapshotContentName = tag.Value
			}

		}

		volumeSnapshots = append(volumeSnapshots, snapv1.VolumeSnapshot{
			TypeMeta: volumeSnapshotMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: snapv1.VolumeSnapshotSpec{
				VolumeSnapshotClassName: &input.CRD.Spec.VolumeSnapshotClassName,
				Source: snapv1.VolumeSnapshotSource{
					VolumeSnapshotContentName: volumeSnapshotContentName,
				},
			},
			Status: nil,
		})

		volumeSnapshotContents = append(volumeSnapshotContents, snapv1.VolumeSnapshotContent{
			TypeMeta: volumeSnapshotContentMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name: *volumeSnapshotContentName,
			},
			Spec: snapv1.VolumeSnapshotContentSpec{
				VolumeSnapshotRef: core_v1.ObjectReference{
					Kind:       volumeSnapshotMeta.Kind,
					Namespace:  namespace,
					Name:       name,
					APIVersion: volumeSnapshotMeta.APIVersion,
				},
				DeletionPolicy:          snapv1.VolumeSnapshotContentRetain,
				Driver:                  "ebs.csi.aws.com",
				VolumeSnapshotClassName: &input.CRD.Spec.VolumeSnapshotClassName,
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
