# ebs-snapshot-provision
The `ebs snapshot provision` operator automatically provisions existing Amazon EBS snapshots in current K8S cluster.

## Description
For dynamic creation and provisioning of AWC EBS snapshots to the cluster, the following is used: `aws-ebs-csi-driver` and `external snapshotter`[aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/examples/kubernetes/snapshot). 
This approach is good when we want to create snapshots from existing PVC within the same cluster. 
But if we want to provide previously created AWS snapshots, for example, in a new cluster for their further restoration, 
then the `aws-ebs-csi-driver` and `external snapshotter` do not support such an automated process for a number of reasons.
The ebs-snapshot-provision operator makes this possible via CR:
```yaml
spec:
  # required fields
  clusterName: kodjin-develop # tenant name
  region: eu-north-1 # AWS region
  frequency: 1m # AWS API request frequency to poll the list of snapshots
  volumeSnapshotClassName: ebs-csi-snapshot-class # current CSI snapshot class name
```
## Requirements
* VolumeSnapshotClass parameters:
  ```yaml
  snapshotClasses:
  - name: {{ .Release.Name }}
    # . . .
    parameters:
      tagSpecification_1: "{{`snapshotNamespace={{ .VolumeSnapshotNamespace }}`}}"
      tagSpecification_2: "{{`snapshotName={{ .VolumeSnapshotName }}`}}"
      tagSpecification_3: "{{`snapshotContentName={{ .VolumeSnapshotContentName }}`}}"
  ```
  should contain additional parameters with placeholders for AWS snapshot tags.
* The following releases are enabled in `release.yaml`:
  * ebs-csi-snapshot-controller
  * ebs-csi-controller

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

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
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

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
