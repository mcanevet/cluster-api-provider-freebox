# Changelog

## [0.1.0](https://github.com/mcanevet/cluster-api-provider-freebox/compare/...v0.1.0) (2025-01-08)

### Features

* generate scaffolding ([d571829](https://github.com/mcanevet/cluster-api-provider-freebox/commit/d571829))
* generate Cluster and Machine resources ([fee6151](https://github.com/mcanevet/cluster-api-provider-freebox/commit/fee6151))
* **ci**: Add integration tests for Freebox connectivity ([1b518a5](https://github.com/mcanevet/cluster-api-provider-freebox/commit/1b518a5))

### Documentation

* add conventional commits and update project documentation ([0634602](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0634602))

### Project Status

This is the initial release of the Cluster API Provider Freebox. The project includes:

* ✅ **Project scaffolding** complete with kubebuilder
* ✅ **API types** defined (`FreeboxCluster` and `FreeboxMachine` v1alpha1)
* ✅ **Integration tests** working against real Freebox hardware
* ✅ **Freebox connectivity** verified with VM permissions
* 🔄 **Controllers** scaffolded, implementation in progress

**Note**: This is an alpha release for development purposes. The controllers are not yet implemented.
