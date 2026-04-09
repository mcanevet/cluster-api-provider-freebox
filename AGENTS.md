# Agent Guidelines

## Commit Message Format

This project uses **[Conventional Commits](https://www.conventionalcommits.org/)** for all commit messages. AI agents must format commits following this convention:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Common types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `chore`: Maintenance tasks
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `ci`: CI/CD changes

**Examples:**
- `feat: add support for 0.2 release series in metadata`
- `fix: correct namespace reference in CAPI operator docs`
- `docs: update installation instructions`

## YAGNI Principle (You Aren't Gonna Need It)

When working with AI agents on this project, **always** follow the YAGNI principle:

- **Only implement what is actually required** - Don't add features or fields "just in case"
- **Start with minimal compliance** - Meet the basic contract requirements first
- **Add complexity incrementally** - Only when there's a proven need
- **Avoid over-engineering** - Don't implement optional features unless explicitly requested

### For Cluster API Provider Development

- **Mandatory fields only initially**: Focus on the required contract fields first
- **Optional features later**: Features like failure domains, templates, and clusterctl support should only be added when needed
- **Simple implementations**: Start with the simplest working implementation

This helps keep the codebase lean, maintainable, and focused on actual requirements rather than speculative features.

## Release Version Bumps

Only update `metadata.yaml` for **minor** and **major** releases - NOT for patch releases.

- Patch (e.g., 0.4.0 → 0.4.1): No metadata.yaml update needed
- Minor (e.g., 0.4.0 → 0.5.0): Update metadata.yaml + kustomization.yaml
- Major (e.g., 0.4.0 → 1.0.0): Update metadata.yaml + kustomization.yaml

### Files to Update for Version Bumps

| File | Description |
|------|-------------|
| `metadata.yaml` | Add new version to `releaseSeries` (source of truth at root) |
| `config/manager/kustomization.yaml` | Update `newTag` to the new version (e.g., `v0.4.1`) |

### Version Bump Checklist

1. **Patch version bump** (e.g., 0.4.0 → 0.4.1):
   - Only update `config/manager/kustomization.yaml` to new version
   - No metadata.yaml update needed (patch versions don't change contract)

2. **Minor version bump** (e.g., 0.4.0 → 0.5.0):
   - Add new version to `metadata.yaml` at root (add to releaseSeries at the top)
   - Update `config/manager/kustomization.yaml` to new version

3. **Major version bump** (e.g., 0.4.0 → 1.0.0):
   - Add new version to `metadata.yaml` at root
   - Update `config/manager/kustomization.yaml` to new version
   - Preserve old versions in releaseSeries for backward compatibility

### Example: Bumping from v0.4.0 to v0.4.1 (patch)

```bash
# Update config/manager/kustomization.yaml
# Change: newTag: v0.4.0 → newTag: v0.4.1
# No metadata.yaml change needed
```

### Example: Bumping from v0.4.0 to v0.5.0 (minor)

```bash
# Update metadata.yaml - add to releaseSeries at the top:
#   - major: 0
#     minor: 5
#     contract: v1beta1

# Update config/manager/kustomization.yaml
# Change: newTag: v0.4.0 → newTag: v0.5.0

# Run make release to verify config/release/metadata.yaml is correct
make release VERSION=v0.5.0
```
