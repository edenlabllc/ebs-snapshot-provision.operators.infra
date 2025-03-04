# EBS snapshot provision operator

[![Release](https://img.shields.io/github/v/release/edenlabllc/ebs-snapshot-provision.operators.infra.svg?style=for-the-badge)](https://github.com/edenlabllc/ebs-snapshot-provision.operators.infra/releases/latest)
[![Software License](https://img.shields.io/github/license/edenlabllc/ebs-snapshot-provision.operators.infra.svg?style=for-the-badge)](LICENSE)
[![Powered By: Edenlab](https://img.shields.io/badge/powered%20by-edenlab-8A2BE2.svg?style=for-the-badge)](https://edenlab.io)

The **EBS Snapshot Provision Operator** automates the provisioning of Amazon EBS snapshots into an existing Kubernetes
cluster. This allows seamless restoration of volumes from snapshots that were originally created in other clusters.

The operator is a small tool that carefully brings snapshots from **the past** and makes them part of **a new future**.

## Description

For dynamic creation and provisioning of AWS EBS snapshots within the **same** Kubernetes cluster, the standard approach
relies on the following components:

- [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver)
- [external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)

This standard approach works well when snapshots are created manually from a PersistentVolumeClaim (PVC) within the same
cluster.

### What problem does this operator solve?

When you need to **automatically provision AWS EBS snapshots created in one cluster into a different cluster**
(e.g., for backup restoration into a new environment), the `aws-ebs-csi-driver` and `external-snapshotter` do not
support this process directly.

The **EBS Snapshot Provision Operator** addresses this gap by watching for AWS EBS snapshots matching specific tags and
automatically creating the corresponding `VolumeSnapshot` and `VolumeSnapshotContent` Kubernetes resources in the target
cluster.

## Key features

- Automatically detects and provisions EBS snapshots created in other clusters.
- Supports custom tagging for identifying snapshots.
- Periodically polls the AWS API to discover new snapshots.
- Automatically creates the required Kubernetes resources (`VolumeSnapshot` and `VolumeSnapshotContent`).

## Custom Resource Specification

The operator is configured via a Custom Resource (CR), which defines the snapshot import policy:

```yaml
spec:
  # Required fields
  clusterName: deps-develop                        # Tenant/Cluster name from which snapshots originate
  region: us-east-1                                # AWS region where snapshots are stored
  frequency: 1m                                    # Polling frequency for AWS API (e.g., 1 minute)
  volumeSnapshotClassName: ebs-csi-snapshot-class  # Name of the VolumeSnapshotClass to use
```

## Requirements

### VolumeSnapshotClass configuration

The target cluster must define a `VolumeSnapshotClass` with parameters that enable tagging of snapshots.  
The following example shows the required placeholders that allow the operator to map AWS snapshot tags to the
corresponding `VolumeSnapshot` and `VolumeSnapshotContent` resources:

```yaml
snapshotClasses:
  - name: ebs-csi-snapshot-class
    parameters:
      tagSpecification_1: "{{`snapshotNamespace={{ .VolumeSnapshotNamespace }}`}}"
      tagSpecification_2: "{{`snapshotName={{ .VolumeSnapshotName }}`}}"
      tagSpecification_3: "{{`snapshotContentName={{ .VolumeSnapshotContentName }}`}}"
```

This tagging scheme allows the operator to correctly match AWS snapshots to Kubernetes objects in the target cluster.

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for
testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever
cluster `kubectl cluster-info` shows).

### Running on the cluster

1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/ebs_v1alpha1_ebssnapshotprovision.yaml
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/core.ebs-snapshot-provision.operators.infra:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/core.ebs-snapshot-provision.operators.infra:tag
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller

UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing

### How it works

This project aims to follow the
Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the
cluster.

### Test It Out

1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)
