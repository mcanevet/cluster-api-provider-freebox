# cluster-api-provider-freebox

Cluster API Provider Freebox is a Kubernetes infrastructure provider for [Cluster API](https://cluster-api.sigs.k8s.io/). It enables you to manage Kubernetes clusters on Freebox hardware using declarative APIs and standard Cluster API workflows.

## Description

This project implements the Cluster API infrastructure provider contract for Freebox. It allows you to provision, manage, and upgrade Kubernetes clusters on Freebox devices, integrating with the Freebox API for device management and networking.

## Getting Started

### Prerequisites
- Go v1.24.0+
- Docker v17.03+
- kubectl v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

### Sample Manifests

Sample manifests for clusters and providers are available in the `talos-example/` directory. See `talos-example/cluster.yaml` for a real-world example.

### Installing with clusterctl (Recommended)

You can install the Freebox infrastructure provider using [clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/overview.html):

1. Create a provider configuration file at `~/.cluster-api/clusterctl.yaml`:

   ```yaml
   providers:
     - name: "freebox"
       url: "https://github.com/mcanevet/cluster-api-provider-freebox/releases/latest/infrastructure-components.yaml"
       type: "InfrastructureProvider"
   ```

2. Initialize Cluster API with the Freebox provider (and your chosen bootstrap/control plane providers):

   ```sh
   clusterctl init -b talos -c talos -i freebox
   ```

   Replace `talos` with your preferred bootstrap/control plane provider if needed.

3. Wait for all providers to be installed and ready.

4. You can now create clusters using the Freebox provider. See `talos-example/cluster.yaml` for an example manifest.

 > **Note:** You must create a Kubernetes Secret and ConfigMap with your Freebox API credentials in the provider namespace. See the provider documentation for details.

**Note:** If you encounter errors about provider release series, ensure you are using a recent release and that the metadata.yaml includes the correct release series for your version.

### To Deploy on the cluster (Manual)
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/cluster-api-provider-freebox:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/cluster-api-provider-freebox:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

For clusterctl users, you can delete providers and clusters using:

```sh
clusterctl delete --all
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/cluster-api-provider-freebox:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/cluster-api-provider-freebox/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

Contributions are welcome! Please follow these guidelines:
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (see AGENTS.md for details)
- Follow the YAGNI principle: only implement what is required
- See AGENTS.md for more agent guidelines
- Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## References
- [Cluster API Documentation](https://cluster-api.sigs.k8s.io/)
- [Freebox Provider Examples](talos-example/)
- [Freebox API Documentation](https://dev.freebox.fr/sdk/os/)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

