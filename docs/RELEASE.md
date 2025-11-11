# Release Process

This project uses [Release Please](https://github.com/googleapis/release-please) to automate releases based on [Conventional Commits](https://www.conventionalcommits.org/).

## How It Works

1. **Commit with conventional commit messages** to `main`:
   - `feat:` - Bumps minor version (0.1.0 → 0.2.0)
   - `fix:` - Bumps patch version (0.1.0 → 0.1.1)
   - `feat!:` or `fix!:` - Bumps major version (breaking change)

2. **Release Please creates/updates a release PR** automatically:
   - Updates CHANGELOG.md
   - Updates `.release-please-manifest.json`
   - The PR title follows conventional commits format

3. **When you merge the release PR**:
   - Release Please creates a GitHub release
   - The release tag triggers the build workflow

4. **The build workflow**:
   - Builds multi-arch container images (amd64, arm64)
   - Pushes images to `ghcr.io/mcanevet/cluster-api-provider-freebox:VERSION`
   - Generates `infrastructure-components.yaml` and `metadata.yaml`
   - Uploads artifacts to the GitHub release

## Installing the Provider

Users can install the provider using clusterctl:

```bash
# Add the provider to clusterctl config
cat >> ~/.cluster-api/clusterctl.yaml <<EOF
providers:
  - name: "freebox"
    url: "https://github.com/mcanevet/cluster-api-provider-freebox/releases/latest/infrastructure-components.yaml"
    type: "InfrastructureProvider"
EOF

# Initialize the provider
clusterctl init --infrastructure freebox
```

Or install directly with kubectl:

```bash
kubectl apply -f https://github.com/mcanevet/cluster-api-provider-freebox/releases/latest/download/infrastructure-components.yaml
```

## Manual Release (if needed)

If you need to create a release manually:

1. Ensure all changes are committed and pushed
2. Create and push a tag:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Create a GitHub release from the tag
4. The build workflow will automatically run and upload artifacts

## Testing the Release Process

You can test the release artifacts generation locally:

```bash
make release VERSION=v0.1.0-test IMG=ghcr.io/mcanevet/cluster-api-provider-freebox:v0.1.0-test
```

This creates artifacts in `config/release/` for inspection.
