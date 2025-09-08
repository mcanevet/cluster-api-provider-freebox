# AGENTS.md

Instructions for AI coding agents working on the Cluster API Provider Freebox project.

## Project Overview

This is a Kubernetes Cluster API infrastructure provider for Freebox Delta virtual machines. It's written in Go and follows Cluster API patterns and conventions.

## Key Technologies

- **Go 1.21+**: Primary language
- **Kubernetes**: Target platform
- **Cluster API**: Framework for cluster lifecycle management
- **Freebox API**: Infrastructure target via [free-go](https://github.com/NikolaLohinski/free-go) client
- **mise**: Tool and environment management
- **controller-runtime**: Kubernetes controller framework

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

- **Unit Tests**: `make test` - Test individual components and logic
- **Integration Tests**: `make test-integration` - Requires Freebox credentials in environment
- **E2E Tests**: `make test-e2e` - Full cluster lifecycle testing
- **Linting**: `make lint` - Go linting with golangci-lint
- **Coverage**: `make test-cover` - Generate coverage reports
- Always add tests for new controllers, API types, and Freebox client functions
- Mock Freebox API calls in unit tests, use real API in integration tests
- Test both success and error scenarios

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

- Title format: `[area] Brief description` (e.g., `[controller] Add FreeboxMachine reconciler`)
- Always run `make lint`, `make test`, and `make build` before committing
- Include integration tests for new Freebox API functionality
- Update documentation for user-facing changes
- Add or update CRD samples in `config/samples/`
- Ensure backwards compatibility or document breaking changes

## Security Considerations

- Never commit Freebox credentials or tokens
- Use Kubernetes secrets for credential management in production
- Validate all user inputs in webhook validators
- Implement proper RBAC for cluster roles
- Sanitize sensitive data in logs and error messages

## Debugging Tips

- Set `LOG_LEVEL=debug` for verbose controller logging
- Use `kubectl describe` to check resource status and events
- Monitor controller logs with `kubectl logs -f deployment/cluster-api-provider-freebox-controller-manager`
- Use Freebox web interface to verify VM and network state
- Check DHCP leases and port forwarding rules for networking issues
