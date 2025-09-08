# Cluster API Provider Freebox

A [Cluster API](https://cluster-api.sigs.k8s.io/) infrastructure provider for managing Kubernetes clusters on Freebox Delta virtual machines.

## Overview

This provider enables you to use your Freebox Delta as infrastructure for running Kubernetes clusters through Cluster API. It leverages the Freebox's built-in virtualization capabilities to create and manage virtual machines that serve as Kubernetes nodes.

## Project Status

🚧 **Under Active Development** - This project is currently being developed using Test-Driven Development (TDD) practices.

### Current State

- ✅ **Project scaffolding** complete with kubebuilder
- ✅ **API types** defined (`FreeboxCluster` and `FreeboxMachine` v1alpha1)
- ✅ **Integration tests** working against real Freebox hardware
- ✅ **Freebox connectivity** verified with VM permissions
- 🔄 **Controllers** scaffolded, implementation in progress
- 🔄 **CRD validation** and webhook development
- 🔄 **Cluster lifecycle** management

### Verified Capabilities

- ✅ Freebox API authentication and authorization
- ✅ VM resource management (tested with 3 CPUs, 15GB RAM, 4 SATA ports)
- ✅ Virtual machine listing and inspection

## Features

- **VM Lifecycle Management**: Create, update, and delete virtual machines on Freebox Delta
- **Network Configuration**: Automatic DHCP lease management and port forwarding setup
- **LAN Integration**: Seamless integration with your existing home network
- **Resource Management**: Efficient allocation of CPU, memory, and storage resources
- **Authentication**: Secure API access using Freebox application tokens

## Prerequisites

### For Users

- Freebox Delta or compatible model with virtualization support
- Freebox OS v4.2+ (API v8+)
- kubectl and clusterctl for cluster management

### For Development

- Go 1.24+ (managed via mise)
- [mise](https://mise.jdx.dev/) for tool and environment management
- kubebuilder for API scaffolding
- Access to a Freebox with VM capabilities for integration testing

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
FREEBOX_VERSION = "latest"

[tools]
go = "1.25.1"
kubectl = "latest"
clusterctl = "latest"
kubebuilder = "latest"
kustomize = "latest"
golangci-lint = "latest"
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

This project follows Test-Driven Development (TDD) practices with integration tests running against real Freebox hardware.

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/mcanevet/cluster-api-provider-freebox.git
cd cluster-api-provider-freebox

# Install tools and set up environment
mise install
mise trust

# Set up your Freebox credentials
cp .mise.local.toml.example .mise.local.toml
# Edit .mise.local.toml with your actual Freebox credentials

# Verify setup
mise current
```

### TDD Workflow

```bash
# Run unit tests
make test

# Run integration tests (requires Freebox credentials)
make test-integration

# Run linting
make lint

# Build the project
make build

# Generate manifests and code
make manifests generate
```

### Project Structure

```text
├── api/v1alpha1/              # CRD definitions
│   ├── freeboxcluster_types.go
│   └── freeboxmachine_types.go
├── internal/controller/       # Reconciler logic
│   ├── freeboxcluster_controller.go
│   └── freeboxmachine_controller.go
├── test/integration/          # Integration tests against real Freebox
│   └── freebox_test.go
├── config/                    # Kubernetes manifests
│   ├── crd/bases/             # Generated CRDs
│   ├── rbac/                  # RBAC roles
│   └── samples/               # Example resources
└── cmd/                       # Main application entry point
```

### Running Tests

```bash
# Unit tests only
make test

# Integration tests (requires FREEBOX_* environment variables)
make test-integration

# E2E tests
make test-e2e

# Test coverage
make test && go tool cover -html=cover.out
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

We welcome contributions! This project follows [Conventional Commits](https://www.conventionalcommits.org/) for consistent commit messages and uses Test-Driven Development (TDD) practices.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes following TDD practices
4. Use conventional commit messages (e.g., `feat(controller): add VM lifecycle management`)
5. Add tests for new functionality
6. Run the full test suite (`make test test-integration`)
7. Submit a pull request

### Commit Message Format

Use conventional commits for all changes:

- `feat(scope): description` for new features
- `fix(scope): description` for bug fixes
- `docs: description` for documentation changes
- `test(scope): description` for test additions/changes

For detailed guidelines, see [AGENTS.md](AGENTS.md).

## Releases

This project uses automated release management:

- **[Release Please](https://github.com/googleapis/release-please)** automatically creates releases based on conventional commits
- **Semantic versioning** is determined from commit message types
- **Changelogs** are auto-generated from the commit history
- **Docker images** are built and published to GitHub Container Registry
- **Release artifacts** include manifests and installation scripts

To trigger a release, simply merge conventional commits to the main branch. Release Please will automatically:

1. Determine the next version number
2. Create a release PR with updated changelog
3. Upon merge, create a GitHub release with built artifacts

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
