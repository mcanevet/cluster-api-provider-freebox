# Release Process

This document describes the release process for the Freebox Cluster API provider.

## Automated Release with Release Please

This project uses [Release Please](https://github.com/googleapis/release-please) to automate releases based on [Conventional Commits](https://www.conventionalcommits.org/).

### How It Works

1. **Commit your changes** using conventional commit messages:

   ```bash
   git commit -m "feat: add support for custom disk types"
   git commit -m "fix: correct IP address detection for IPv6"
   git commit -m "docs: update installation instructions"
   ```

2. **Push to main branch** - Release Please creates a PR automatically with version bump and CHANGELOG

3. **Review and merge the release PR** - This triggers the automated release:
   - Tags the release
   - Creates GitHub release
   - Builds and pushes container image
   - Uploads release artifacts

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New features (triggers minor version bump)
- `fix:` - Bug fixes (triggers patch version bump)
- `feat!:` or `fix!:` - Breaking changes (triggers major version bump)
- `docs:`, `test:`, `ci:`, `build:` - Other changes (no version bump)

### What Gets Automated

The GitHub Action automatically:

- ✅ Creates and updates release PR
- ✅ Generates CHANGELOG.md
- ✅ Bumps version in Makefile
- ✅ Creates git tag when PR is merged
- ✅ Builds multi-arch container image
- ✅ Pushes image to ghcr.io
- ✅ Generates and uploads release artifacts

### Bootstrap the First Release

For the very first release, create an initial conventional commit:

```bash
git commit --allow-empty -m "feat: initial freebox infrastructure provider release

- Support for VM lifecycle management
- Automatic image download and preparation
- Support for both Kubeadm and Talos bootstrap providers
- CAPI v1beta2 contract compliance"

git push origin main
```

Release Please will create a PR bumping to `v0.1.0`. Review and merge it.

---

## Manual Release Process (Fallback)

If the automation fails, follow these manual steps:

### Prerequisites

1. **Container Registry Access**: You need push access to `ghcr.io/mcanevet/cluster-api-provider-freebox`
2. **GitHub Repository Access**: You need write access to create releases and tags
3. **Clean Working Directory**: Ensure all changes are committed

### Manual Release Steps

### 1. Update Version

The version is defined in the `Makefile`:

```makefile
VERSION ?= v0.1.0
```

### 2. Generate Release Artifacts

Run the release target to generate `infrastructure-components.yaml` and `metadata.yaml`:

```bash
make release
```

This will:

- Build all manifests with `make manifests`
- Set the controller image to `ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0`
- Generate `config/release/infrastructure-components.yaml` (all-in-one deployment manifest)
- Generate `config/release/metadata.yaml` (provider metadata for clusterctl)

### 3. Review Release Artifacts

Check the generated files:

```bash
cat config/release/metadata.yaml
wc -l config/release/infrastructure-components.yaml
```

Ensure:

- `metadata.yaml` has the correct version and contract
- `infrastructure-components.yaml` contains all CRDs, RBAC, and controller deployment
- All resources are properly labeled with `cluster.x-k8s.io/provider=infrastructure-freebox`

### 4. Build and Push Container Image

Build the multi-platform container image:

```bash
# For local testing
make docker-build IMG=ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0

# For production release (requires authenticated ghcr.io access)
make docker-buildx IMG=ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0
```

Push the image:

```bash
make docker-push IMG=ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0
```

### 5. Commit Release Artifacts

```bash
git add config/release/
git add Makefile  # if version changed
git commit -m "Release v0.1.0"
```

### 6. Create and Push Git Tag

```bash
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

### 7. Create GitHub Release

1. Go to <https://github.com/mcanevet/cluster-api-provider-freebox/releases/new>

2. Fill in the release form:
   - **Tag**: `v0.1.0` (select the tag you just pushed)
   - **Release title**: `v0.1.0`
   - **Description**: See release notes template below

3. **Attach release assets**:
   - Upload `config/release/infrastructure-components.yaml`
   - Upload `config/release/metadata.yaml`

4. Click "Publish release"

### Release Notes Template

```markdown
# Release v0.1.0

## Features

- ✨ Initial release of Freebox infrastructure provider for Cluster API
- 🚀 Support for VM lifecycle management (create, start, stop, delete)
- 📥 Automatic image download and preparation from URLs (supports .raw, .qcow2, compressed formats)
- 🔧 Support for both Kubeadm and Talos bootstrap providers
- ✅ CAPI v1beta2 contract compliance
- 🎯 Ready condition lifecycle tracking
- 🔍 Automatic IP address discovery via Freebox LAN browser

## Installation

### Using kubectl directly

```bash
kubectl apply -f https://github.com/mcanevet/cluster-api-provider-freebox/releases/download/v0.1.0/infrastructure-components.yaml
```

### Configuring credentials

Create a secret with your Freebox API credentials:

```bash
kubectl create secret generic cluster-api-provider-freebox-freebox-secret \
  --from-literal=token="${FREEBOX_TOKEN}" \
  -n cluster-api-provider-freebox-system

kubectl create configmap cluster-api-provider-freebox-freebox-config \
  --from-literal=endpoint="${FREEBOX_ENDPOINT}" \
  --from-literal=version="${FREEBOX_VERSION}" \
  --from-literal=app_id="${FREEBOX_APP_ID}" \
  --from-literal=device="${FREEBOX_DEVICE}" \
  -n cluster-api-provider-freebox-system
```

## Documentation

- [Talos Cluster Example](talos-example/) - Deploy a Talos-based Kubernetes cluster
- [API Reference](api/v1alpha1/) - FreeboxCluster and FreeboxMachine API documentation

## Container Images

- `ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0`

## What's Next

See the [talos-example](talos-example/) directory for a complete example of deploying a Kubernetes cluster on Freebox using Talos.

## Compatibility

- **Cluster API**: v1.11.x (v1beta2 contract)
- **Kubernetes**: 1.33+ (tested with 1.34.1)
- **Freebox**: Delta and newer (tested with API v10)

```

## Verification

After the release:

1. Verify the GitHub release is published
2. Test installation:
   ```bash
   kubectl apply -f https://github.com/mcanevet/cluster-api-provider-freebox/releases/download/v0.1.0/infrastructure-components.yaml
   ```

3. Verify the controller starts successfully
4. Test creating a cluster using the Talos example

## Troubleshooting

### Image not found

If the controller image is not found, ensure:

- The image was pushed to ghcr.io
- The image is public (or RBAC is configured for private access)
- The image tag matches the version in `infrastructure-components.yaml`

### Release assets not found

Ensure:

- The tag exists in GitHub
- The release assets were uploaded correctly
- The URLs match the pattern: `https://github.com/mcanevet/cluster-api-provider-freebox/releases/download/v0.1.0/<filename>`
