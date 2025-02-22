# EBS snapshot provision operator

[![Release](https://img.shields.io/github/v/release/edenlabllc/ebs-snapshot-provision.operators.infra.svg?style=for-the-badge)](https://github.com/edenlabllc/ebs-snapshot-provision.operators.infra/releases/latest)
[![Software License](https://img.shields.io/github/license/edenlabllc/ebs-snapshot-provision.operators.infra.svg?style=for-the-badge)](LICENSE)
[![Powered By: Edenlab](https://img.shields.io/badge/powered%20by-edenlab-8A2BE2.svg?style=for-the-badge)](https://edenlab.io)

The EBS snapshot provision operator automatically provisions Amazon EBS snapshots to be used in an existing K8S cluster.

## Description

For dynamic creation and provisioning of AWC EBS snapshots for a K8S cluster, the following components can be
used: [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/examples/kubernetes/snapshot).
and [external snapshotter](https://github.com/kubernetes-csi/external-snapshotter)
This approach is good when we want to create the snapshots manually from an existing PVC within the same cluster.
However, if we want to automatically provision AWS EBS snapshots and then use in, for example, in a new K8S cluster for
further
restoration,
then the `aws-ebs-csi-driver` and `external-snapshotter` do not support such an automated process for a number of
reasons.
The EBS snapshot provision operator makes this possible via the CR:

```yaml
spec:
  # required fields
  clusterName: deps-develop # tenant name
  region: us-east-1 # AWS region
  frequency: 1m # AWS API request frequency to poll the list of snapshots
  volumeSnapshotClassName: ebs-csi-snapshot-class # current CSI snapshot class name
```

## Requirements

* The `VolumeSnapshotClass` parameters are:
  ```yaml
  snapshotClasses:
  - name: ebs-csi-snapshot-class
    # . . .
    parameters:
      tagSpecification_1: "{{`snapshotNamespace={{ .VolumeSnapshotNamespace }}`}}"
      tagSpecification_2: "{{`snapshotName={{ .VolumeSnapshotName }}`}}"
      tagSpecification_3: "{{`snapshotContentName={{ .VolumeSnapshotContentName }}`}}"
  ```
  should contain additional parameters with placeholders for AWS snapshot tags.

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
