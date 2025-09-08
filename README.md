# Cluster API Provider Freebox

A [Cluster API](https://cluster-api.sigs.k8s.io/) infrastructure provider for managing Kubernetes clusters on Freebox Delta virtual machines.

## Overview

This provider enables you to use your Freebox Delta as infrastructure for running Kubernetes clusters through Cluster API. It leverages the Freebox's built-in virtualization capabilities to create and manage virtual machines that serve as Kubernetes nodes.

## Features

- **VM Lifecycle Management**: Create, update, and delete virtual machines on Freebox Delta
- **Network Configuration**: Automatic DHCP lease management and port forwarding setup
- **LAN Integration**: Seamless integration with your existing home network
- **Resource Management**: Efficient allocation of CPU, memory, and storage resources
- **Authentication**: Secure API access using Freebox application tokens

## Prerequisites

- Freebox Delta or compatible model with virtualization support
- Freebox OS v4.2+ (API v8+)
- Go 1.21+ for development
- kubectl and clusterctl for cluster management
- mise for tool and environment management

## Quick Start

### 1. Generate Freebox API Credentials

First, you'll need to generate API credentials for your Freebox. Follow the [credential generation guide](https://nikolalohinski.github.io/terraform-provider-freebox/provider.html#generating-credentials) using the terraform provider's CLI tool.

### 2. Configure Environment Variables

This project uses [mise](https://mise.jdx.dev/) for tool and environment management. The configuration is split between commitable and sensitive variables:

**Step 1**: The repository includes `.mise.toml` with safe, commitable environment variables.

**Step 2**: Create a local configuration file for sensitive credentials:

```bash
# Copy the template
cp .mise.local.toml.example .mise.local.toml

# Edit with your actual Freebox credentials
# FREEBOX_APP_ID="your-actual-app-id"
# FREEBOX_TOKEN="your-actual-private-token"
```

**Step 3**: Install tools and activate environment:

```bash
mise install
mise trust
```

### 3. Install the Provider

```bash
# Install clusterctl if not already installed
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/latest/download/clusterctl-linux-amd64 -o clusterctl
sudo install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl

# Initialize Cluster API with the Freebox provider
clusterctl init --infrastructure freebox
```

### 4. Create Your First Cluster

```bash
# Generate cluster configuration
clusterctl generate cluster my-freebox-cluster --infrastructure freebox > cluster.yaml

# Apply the configuration
kubectl apply -f cluster.yaml

# Get the kubeconfig for your new cluster
clusterctl get kubeconfig my-freebox-cluster > kubeconfig-my-freebox-cluster.yaml
```

## Configuration

### Environment Variables (via mise)

This project uses [mise](https://mise.jdx.dev/) for tool and environment management with a two-file approach:

**`.mise.toml`** (commitable - safe environment variables):

```toml
[env]
FREEBOX_ENDPOINT = "mafreebox.freebox.fr"
FREEBOX_VERSION = "v10"

[tools]
go = "1.21"
kubectl = "latest"
clusterctl = "latest"
```

**`.mise.local.toml`** (not commitable - sensitive credentials):

```toml
[env]
FREEBOX_APP_ID = "{{ your-actual-app-id }}"
FREEBOX_TOKEN = "{{ your-actual-private-token }}"
```

The `.mise.local.toml` file is excluded from git via `.gitignore` to keep credentials secure.

### Cluster Configuration Example

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-freebox-cluster
spec:
  clusterNetwork:
    pods:
      cidrBlocks: ["192.168.0.0/16"]
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: FreeboxCluster
    name: my-freebox-cluster
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: FreeboxCluster
metadata:
  name: my-freebox-cluster
spec:
  endpoint: "mafreebox.freebox.fr"
  version: "v10"
  network:
    subnet: "192.168.1.0/24"
    gateway: "192.168.1.1"
```

## Development

### Setup Development Environment

```bash
# Install mise if not already installed
curl https://mise.run | sh

# Install tools and set up environment
mise install
mise trust

# Verify setup
mise current
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/mcanevet/cluster-api-provider-freebox.git
cd cluster-api-provider-freebox

# Build the provider
make build

# Run tests
make test

# Build and load Docker image for development
make docker-build
```

### Running Tests

```bash
# Unit tests
make test

# Integration tests (requires Freebox credentials)
make test-integration

# E2E tests
make test-e2e
```

## Architecture

The provider consists of several key components:

- **FreeboxCluster**: Manages cluster-wide infrastructure on Freebox
- **FreeboxMachine**: Manages individual virtual machines
- **Controllers**: Reconcile desired state with actual Freebox resources
- **Freebox Client**: Wraps the [free-go](https://github.com/NikolaLohinski/free-go) library

### Supported Resources

- Virtual Machine creation and management
- DHCP lease management
- Port forwarding configuration
- Network interface management
- LAN host configuration

## Troubleshooting

### Common Issues

1. **Authentication Failures**: Ensure your Freebox app token is valid and has sufficient permissions
2. **Network Connectivity**: Verify your Freebox is accessible from the machine running the provider
3. **Resource Limits**: Check available resources on your Freebox (CPU, memory, storage)

### Debug Mode

Enable debug logging:

```bash
export LOG_LEVEL=debug
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the full test suite
6. Submit a pull request

## Security

This provider handles sensitive Freebox API credentials. Always:

- Store credentials securely (use environment variables, not hardcoded values)
- Rotate API tokens regularly
- Limit API permissions to minimum required
- Keep the provider updated to latest version

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Cluster API](https://cluster-api.sigs.k8s.io/) community for the excellent framework
- [free-go](https://github.com/NikolaLohinski/free-go) for the Freebox API client library
- Free/Iliad for providing the Freebox platform and API documentation

## Related Projects

- [terraform-provider-freebox](https://github.com/NikolaLohinski/terraform-provider-freebox) - Terraform provider for Freebox
- [free-go](https://github.com/NikolaLohinski/free-go) - Go client library for Freebox API
- [Cluster API](https://github.com/kubernetes-sigs/cluster-api) - Kubernetes cluster lifecycle management
