# AGENTS.md - Meta Instructions

## Core Development Principles

### YAGNI (You Ain't Gonna Need It)

- Only implement what you actually need right now
- Avoid adding features "just in case" or for potential future use
- Skip optional components until they become necessary
- Examples: webhooks for simple validation, complex networking for single VM setup

### Minimal Code Philosophy

- Always generate the minimum amount of code necessary for each change
- Make small, reviewable changes that can be easily understood
- Avoid large code blocks or comprehensive configurations in single edits
- Remove unnecessary code and fields that don't add value
- Prefer simple, local implementations over heavy external dependencies

### Resource-Constrained Development

- Design for single VM deployment scenarios
- Optimize for minimal resource usage
- Avoid complex infrastructure assumptions
- Keep API types simple and focused on essential fields only

## Development Workflow

### Test-Driven Development (TDD)

- Write tests first, then implement functionality
- Focus on testing business logic, not simple data structures
- Skip tests for API types unless they contain complex validation logic
- Test controllers where the real business logic lives

### Version Management

- Always verify latest version of tools before adding to mise.toml
- Use stable, latest versions for production readiness
- Document version choices and rationale

### Commit Standards

- Use Conventional Commits for commit messages
- Make atomic commits with clear, descriptive messages
- Follow semantic versioning principles

## Project Context

### Overview

Creating a Cluster API infrastructure provider for Freebox following the official getting started guide.

### Constraints

- Single VM deployment scenario (resource limitations)
- Minimal dependencies and complexity
- Contract compliance with Cluster API v1beta2
- Integration with free-go library for Freebox API access

### Architecture Decisions Made

- API version: v1alpha1 (appropriate for early development)
- Scheme registration: Minimal pattern without controller-runtime dependency
- API fields: Only essential fields (ProviderID, VM config, initialization status)
- Contract compliance: Implemented required fields only (initialization.provisioned)

## Implementation Guidelines

### API Design

- Follow Cluster API contract requirements (v1beta2)
- Use minimal field sets focused on actual needs
- Implement only mandatory contract fields
- Defer optional fields until proven necessary

### Dependency Management

- Minimize external dependencies in API packages
- Use local type definitions instead of heavy imports when possible
- Prefer Kubernetes core types over provider-specific libraries in APIs

### Tool Verification

- go: 1.25.1 (latest stable)
- kubebuilder: 4.8.0 (latest)
- kubectl: 1.34.0 (latest)
- kustomize: 5.7.1 (latest)

## Current Status

- ✅ Project scaffolding complete
- ✅ API types implemented with contract compliance
- ✅ Minimal scheme registration pattern implemented
- ✅ VM configuration fields added (CPUs, Memory, DiskSize, ImageURL)
- ✅ free-go dependency integrated for Freebox API access
- 🔄 Next: Controller implementation for actual VM provisioning logic
