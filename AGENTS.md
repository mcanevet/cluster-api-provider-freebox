# AGENTS.md

Instructions for AI coding agents working on the Cluster API Provider Freebox project.

## Project Overview

This is a Kubernetes Cluster API infrastructure provider for Freebox Delta virtual machines. The project is currently under active development using Test-Driven Development (TDD) practices. It's written in Go and follows Cluster API patterns and conventions.

## Current Development Status

### Completed ✅

- **Project scaffolding** with kubebuilder and Cluster API domain
- **API types** defined (`FreeboxCluster` and `FreeboxMachine` v1alpha1)
- **Integration test framework** with real Freebox connectivity
- **Free-go client integration** for Freebox API access
- **TDD workflow** established with `make test-integration`
- **Environment management** via mise with secure credential handling

### In Progress 🔄

- **Controller implementations** for FreeboxCluster and FreeboxMachine
- **CRD validation** and webhook development
- **VM lifecycle management** (create, update, delete operations)
- **Network configuration** and DHCP lease management

### Verified Capabilities ✅

- Freebox API authentication with VM permissions
- VM resource inspection (3 CPUs, 15GB RAM, 4 SATA ports tested)
- Virtual machine listing and basic operations

## Key Technologies

- **Go 1.24+**: Primary language (managed via mise)
- **Kubernetes v0.33.0**: Target platform
- **Cluster API**: Framework for cluster lifecycle management
- **Kubebuilder**: Project scaffolding and controller framework
- **controller-runtime v0.21.0**: Kubernetes controller framework
- **free-go v1.8.1**: Freebox API client library
- **testify v1.11.1**: Testing framework for unit and integration tests
- **mise**: Tool and environment management
- **ginkgo/gomega**: E2E testing framework

## Development Environment Tips

- Use `mise install` to set up all required tools and Go version
- Environment variables split between `.mise.toml` (commitable) and `.mise.local.toml` (sensitive)
- Run `mise trust` after cloning to activate environment
- Copy `.mise.local.toml` template and add your Freebox credentials
- Use `mise current` to verify tool versions
- Freebox credentials stored in `.mise.local.toml` (never commit this file)
- Follow kubebuilder project structure and patterns

## Project Structure

```text
├── api/v1beta1/          # CRD definitions (FreeboxCluster, FreeboxMachine)
├── internal/controller/  # Reconciler logic
├── internal/freebox/     # Freebox API client wrapper
├── config/              # Kubernetes manifests and deployment configs
├── test/                # Integration and e2e tests
└── hack/                # Build and development scripts
```

## Testing Instructions

### TDD Workflow

- Follow Test-Driven Development: write tests first, then implement functionality
- **Unit Tests**: `make test` - Test individual components with mocked dependencies
- **Integration Tests**: `make test-integration` - Test against real Freebox (requires credentials)
- **E2E Tests**: `make test-e2e` - Full cluster lifecycle testing with Kind
- **Linting**: `make lint` - Go linting with golangci-lint
- **Coverage**: Run `make test && go tool cover -html=cover.out` for coverage reports

### Test Structure

- **Unit tests**: Co-located with source code (e.g., `*_test.go`)
- **Integration tests**: `test/integration/` - Tests that require real Freebox
- **E2E tests**: `test/e2e/` - Full end-to-end cluster testing

### Integration Test Environment

- Tests use environment variables from mise configuration
- Set `INTEGRATION_TESTS=true` to enable integration tests
- Requires valid `FREEBOX_APP_ID` and `FREEBOX_TOKEN` in `.mise.local.toml`
- Tests verify: authentication, VM listing, resource inspection
- Always test both success and error scenarios
- Mock Freebox API calls in unit tests, use real API in integration tests

### Test Guidelines

- Add tests for all new controllers, API types, and Freebox client functions
- Test VM lifecycle operations thoroughly (create, update, delete, start, stop)
- Verify DHCP lease management and network configuration
- Test error handling and edge cases
- Ensure tests are deterministic and can run in parallel

## Code Standards

- Follow standard Go conventions and gofmt formatting
- Use controller-runtime logging with structured logging
- Implement proper error handling and wrapping
- Add comprehensive godoc comments for exported functions
- Follow Kubernetes API conventions for CRD fields
- Use kubebuilder markers for CRD generation
- Implement proper RBAC markers for controllers

## Freebox Integration

- Use the `free-go` client library for all Freebox API interactions
- Implement proper authentication and session management
- Handle API rate limiting and retries gracefully
- Support multiple Freebox API versions when possible
- Cache VM and network state to minimize API calls
- Implement proper cleanup on resource deletion

## Build and Release

- Use `make build` for local builds
- Use `make docker-build` for container images
- Follow semantic versioning for releases
- Update compatibility matrix in README for new releases
- Ensure all tests pass before merging PRs

## PR Instructions

### Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for consistent and semantic commit messages:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:**

- `feat`: New features
- `fix`: Bug fixes
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring without feature changes
- `test`: Adding or updating tests
- `chore`: Maintenance tasks, dependency updates
- `ci`: CI/CD configuration changes

**Scopes (examples):**

- `api`: API types and CRDs
- `controller`: Controller implementations
- `freebox`: Freebox client integration
- `integration`: Integration tests
- `e2e`: End-to-end tests

**Examples:**

```text
feat(controller): implement FreeboxMachine reconciler
fix(freebox): handle VM creation timeout errors
docs(readme): update installation instructions
test(integration): add VM lifecycle tests
chore(deps): update controller-runtime to v0.21.0
```

### PR Guidelines

- **Title format**: Use conventional commit format for PR titles
- **Pre-commit checks**: Always run `make lint`, `make test`, and `make build` before committing
- **Integration tests**: Include integration tests for new Freebox API functionality
- **Documentation**: Update documentation for user-facing changes
- **Examples**: Add or update CRD samples in `config/samples/`
- **Compatibility**: Ensure backwards compatibility or document breaking changes
- **Scope**: Keep PRs focused on a single feature or fix

## Security Considerations

- Never commit Freebox credentials or tokens
- Use Kubernetes secrets for credential management in production
- Validate all user inputs in webhook validators
- Implement proper RBAC for cluster roles
- Sanitize sensitive data in logs and error messages

## Implementation Priorities

### Phase 1: Core VM Management (Current Focus)

1. **FreeboxMachine Controller**:
   - Implement basic VM lifecycle (create, delete, status updates)
   - Handle VM specifications (CPU, memory, disk)
   - Manage VM power states (start, stop, reset)
   - Update machine status with provider ID and addresses

2. **FreeboxCluster Controller**:
   - Basic cluster infrastructure setup
   - Network configuration and endpoint management
   - Status reporting and condition management

3. **Integration Tests**:
   - VM creation and deletion workflows
   - Resource allocation and limits testing
   - Error handling and retry logic

### Phase 2: Advanced Features

1. **Network Management**:
   - DHCP static lease creation/deletion
   - Port forwarding for cluster access
   - Load balancer configuration

2. **Validation and Webhooks**:
   - Resource validation (CPU, memory, disk limits)
   - Admission control for resource requests
   - Defaulting webhooks for common configurations

3. **Observability**:
   - Comprehensive metrics and monitoring
   - Event recording for debugging
   - Structured logging with context

### Next Immediate Steps

1. Start with `FreeboxMachine` controller implementation
2. Write integration tests for VM creation before implementation
3. Use TDD approach: test → implement → refactor cycle
4. Focus on happy path first, then error scenarios
