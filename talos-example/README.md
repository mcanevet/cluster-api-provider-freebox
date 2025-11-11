# Talos Cluster with Freebox Infrastructure Provider

This directory contains example manifests for deploying a single-node Kubernetes cluster using:

- **Infrastructure Provider**: Freebox (cluster-api-provider-freebox)
- **Bootstrap Provider**: Talos (bootstrap.cluster.x-k8s.io)
- **ControlPlane Provider**: Talos (controlplane.cluster.x-k8s.io)

This is a minimal configuration designed for resource-constrained Freebox environments, running a single control plane node that also runs workloads.

## Prerequisites

1. Management cluster with the following providers installed:

   **Note**: Since the Freebox provider is not yet released, you need to install it manually.

   ```bash
   # Create a Kind management cluster
   kind create cluster --name capi-management

   # Initialize with Talos providers
   clusterctl init \
     --bootstrap talos \
     --control-plane talos

   # Install Freebox provider manually
   # From the cluster-api-provider-freebox repository root:
   make docker-build IMG=example.com/cluster-api-provider-freebox:v0.0.1
   kind load docker-image example.com/cluster-api-provider-freebox:v0.0.1 --name capi-management

   # Apply the Freebox provider manifests using kustomize
   kubectl apply -k config/default

   # Update credentials with your actual Freebox credentials
   kubectl create secret generic cluster-api-provider-freebox-freebox-secret \
     --from-literal=token="${FREEBOX_TOKEN}" \
     -n cluster-api-provider-freebox-system \
     --dry-run=client -o yaml | kubectl apply -f -

   kubectl create configmap cluster-api-provider-freebox-freebox-config \
     --from-literal=endpoint="${FREEBOX_ENDPOINT}" \
     --from-literal=version="${FREEBOX_VERSION}" \
     --from-literal=app_id="${FREEBOX_APP_ID}" \
     --from-literal=device="${FREEBOX_DEVICE}" \
     -n cluster-api-provider-freebox-system \
     --dry-run=client -o yaml | kubectl apply -f -

   # Restart the controller to pick up the credentials
   kubectl rollout restart deployment cluster-api-provider-freebox-controller-manager \
     -n cluster-api-provider-freebox-system

   # Fix imagePullPolicy to use the locally loaded image
   kubectl patch deployment cluster-api-provider-freebox-controller-manager \
     -n cluster-api-provider-freebox-system \
     --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}]'

   # Wait for the controller to be ready
   kubectl wait --for=condition=available --timeout=120s \
     deployment/cluster-api-provider-freebox-controller-manager \
     -n cluster-api-provider-freebox-system
   ```

2. Freebox credentials must be set as environment variables:
   - `FREEBOX_TOKEN` - Your Freebox API token
   - `FREEBOX_ENDPOINT` - Freebox API endpoint (e.g., `https://mafreebox.freebox.fr`)
   - `FREEBOX_VERSION` - API version (e.g., `v10`)
   - `FREEBOX_APP_ID` - Your application ID
   - `FREEBOX_DEVICE` - Device name

3. Talos image will be automatically downloaded by the controller from the URL specified in the FreeboxMachineTemplate

## Files

- `cluster.yaml` - Main Cluster and FreeboxCluster resources
- `controlplane.yaml` - TalosControlPlane and FreeboxMachineTemplate for the single control plane node

## Configuration Notes

### FreeboxCluster

- **controlPlaneEndpoint**: Set this to an available IP address on your network that will be used as the control plane endpoint (e.g., 192.168.1.100)

### FreeboxMachineTemplate

The Freebox provider uses the `FreeboxMachineTemplate` to describe how to create VMs. Its spec mirrors `FreeboxMachineSpec`:

- **name**: Desired VM name on the Freebox (must be unique per cluster)
- **vcpus**: Number of virtual CPUs (minimum 1)
- **memoryMB**: RAM size in megabytes (e.g. 4096 for 4GiB)
- **diskSizeBytes**: Target virtual disk size in bytes; the controller will resize the downloaded image up to this size (e.g. `10737418240` for 10GiB)
- **imageURL**: URL to the Talos disk image; the controller will download, (optionally) extract, copy, rename, and resize it automatically.

Example (from `controlplane.yaml`):

```yaml
spec:
  template:
    spec:
      name: talos-cp
      vcpus: 2
      memoryMB: 4096
      diskSizeBytes: 10737418240
      imageURL: https://github.com/siderolabs/talos/releases/download/v1.11.5/metal-arm64.raw.xz
```

Workflow handled by the controller:

1. Download the compressed image to the Freebox download directory
2. Extract (if compressed) or copy to the VM storage directory
3. Rename to `<vm-name><ext>` (e.g. `talos-cp.raw`)
4. Resize the disk to `diskSizeBytes`
5. Create and start the VM, then record `vmID`, `diskPath`, and IP addresses in status.

You DO NOT need a separate image resource. Setting `imageURL` triggers the full lifecycle.

### TalosControlPlane

- **replicas**: Set to 1 for single-node cluster
- **talosVersion**: Version of Talos to use (v1.11.5)
- **generateType**: `controlplane` for control plane nodes
- **configPatches**: Customize Talos configuration
  - Install disk: `/dev/vda` (standard for Freebox VMs)
  - Installer image: `ghcr.io/siderolabs/installer:v1.11.5`
  - CNI: Set to `none` if using a custom CNI (or remove to use default)
  - **allowSchedulingOnControlPlanes**: Set to `true` to allow workloads on the control plane node

## Deployment

1. Pick a control plane endpoint IP and set it in `cluster.yaml`
2. (Optional) Adjust `diskSizeBytes`, `vcpus`, `memoryMB` to fit your Freebox resources
3. Apply the manifests:

   ```bash
   kubectl apply -f cluster.yaml
   kubectl apply -f controlplane.yaml
   ```

## Accessing the Cluster

Once the cluster is provisioned, retrieve the kubeconfig:

```bash
kubectl get secret talos-cluster-kubeconfig -o jsonpath='{.data.value}' | base64 -d > talos-cluster.kubeconfig
```

Then use it to access the cluster:

```bash
export KUBECONFIG=talos-cluster.kubeconfig
kubectl get nodes
```

## Talos-Specific Operations

To interact with Talos on the nodes, you'll need `talosctl`:

```bash
# Get the talosconfig
kubectl get talosconfig -o yaml talos-cluster-cp -o jsonpath='{.status.talosConfig}' > talosconfig

# Use talosctl to interact with nodes
talosctl --talosconfig talosconfig -n <node-ip> dashboard
```

## Notes

- This is a **single-node cluster** with workloads running on the control plane (`allowSchedulingOnControlPlanes: true`)
- The Freebox controller downloads the Talos image automatically; ensure the Freebox has enough free space for both the compressed and expanded image plus resize overhead.
- If the image download fails, check the FreeboxMachine conditions (`kubectl describe freeboxmachine <name>`).
- Unlike kubeadm-based clusters, Talos clusters:
  - Don't use cloud-init (set `cloudInitEnabled: false`)
  - Have immutable, API-driven configuration
  - Use their own bootstrap process
- The control plane endpoint IP must be configured to point to your control plane node
- You may need to configure your network to route traffic to the control plane endpoint
