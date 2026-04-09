# Changelog

## [0.4.2](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.4.1...v0.4.2) (2026-04-09)


### Bug Fixes

* skip VM start if already running to support clusterctl move pivot ([5646697](https://github.com/mcanevet/cluster-api-provider-freebox/commit/564669770c3a08e85c8ae34c049e2270e2cb1706))

## [0.4.1](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.4.0...v0.4.1) (2026-04-08)


### Bug Fixes

* force v0.4.1 in kustomization files ([a72cccb](https://github.com/mcanevet/cluster-api-provider-freebox/commit/a72cccbaf7ea06acbcb5095af30280cd49a6d3f9))

## [0.4.0](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.5...v0.4.0) (2026-04-08)


### Features

* add CAPI pivot compatibility with block-move annotation ([ec160ac](https://github.com/mcanevet/cluster-api-provider-freebox/commit/ec160ac733f88c1b6d022b4b98b2a0c4f60394ce))
* add CAPI pivot compatibility with paused annotation check ([583f663](https://github.com/mcanevet/cluster-api-provider-freebox/commit/583f663be51a27fdcfb1f74f6e3b7f4f1abac3e0))

## [0.3.5](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.4...v0.3.5) (2026-04-08)


### Bug Fixes

* find workload cluster node by IP instead of name for Talos compatibility ([6678f37](https://github.com/mcanevet/cluster-api-provider-freebox/commit/6678f371482e17dc132377641f5bb4ca0a6c9b4a))

## [0.3.4](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.3...v0.3.4) (2026-04-08)


### Bug Fixes

* decouple provisioned=true from workload cluster node patching ([dda2da8](https://github.com/mcanevet/cluster-api-provider-freebox/commit/dda2da89c1fdb5709b5cc9e052e5b1cb45a2967c))
* **deps:** update module sigs.k8s.io/cluster-api to v1.12.5 ([a16c732](https://github.com/mcanevet/cluster-api-provider-freebox/commit/a16c7327464d96d3c7e7c91b0fbadfc76182a40f))
* **deps:** update module sigs.k8s.io/cluster-api/test to v1.12.5 ([0852226](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0852226b3ab7ef37457d43aeb1411f95b6ca7eab))

## [0.3.3](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.2...v0.3.3) (2026-04-07)


### Bug Fixes

* **deps:** update k8s.io/utils digest to 28399d8 ([4d03bc1](https://github.com/mcanevet/cluster-api-provider-freebox/commit/4d03bc1b343bf32bb6748323381989e5ea7a6c6f))
* **deps:** update kubernetes monorepo to v0.35.3 ([3f5d43e](https://github.com/mcanevet/cluster-api-provider-freebox/commit/3f5d43ec7e17b1b405c801295b10b786030ec542))
* **deps:** update module github.com/onsi/ginkgo/v2 to v2.28.1 ([3bb25b3](https://github.com/mcanevet/cluster-api-provider-freebox/commit/3bb25b3a32c39c014ebae00102ad2d6889d60b17))
* **deps:** update module github.com/onsi/gomega to v1.39.1 ([0c16d81](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0c16d81d706d1fefc641185ff2190f8a287bea03))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.23.3 ([89fc095](https://github.com/mcanevet/cluster-api-provider-freebox/commit/89fc0953e794401a9b0fbf339f54a3228c4ac109))

## [0.3.2](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.1...v0.3.2) (2026-04-07)


### Bug Fixes

* enable Machine Ready by patching Node with providerID ([202566c](https://github.com/mcanevet/cluster-api-provider-freebox/commit/202566c68f64140e53b9fc55a6c84cff830d12ed))

## [0.3.1](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.3.0...v0.3.1) (2026-04-05)


### Bug Fixes

* deduplicate download tasks and clean up downloads after copy/extract ([0df8599](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0df85992c8e2946be2c035d7c178a1be266bc220))
* deduplicate VM creation and handle empty ListVirtualMachines response ([8e243e3](https://github.com/mcanevet/cluster-api-provider-freebox/commit/8e243e38e705609da9015df3bb2cdc1b06a75182))
* handle errors from Status().Update() instead of silencing them ([bda8bc2](https://github.com/mcanevet/cluster-api-provider-freebox/commit/bda8bc280b79d02243fce94a3f0bae90488c30ec))
* migrate e2e tests from v1beta1 to v1beta2 CAPI APIs ([9676d6a](https://github.com/mcanevet/cluster-api-provider-freebox/commit/9676d6ac5298cede7047674e51a22886d3df60b6))
* replace blocking time.Sleep VM-stop loop with RequeueAfter in deletion path ([b30b3b5](https://github.com/mcanevet/cluster-api-provider-freebox/commit/b30b3b5edfa58f1b605d83928f4145e17c4fa017))
* replace hardcoded "default" with clusterProxy.GetName() in e2e GetIntervals calls ([284178c](https://github.com/mcanevet/cluster-api-provider-freebox/commit/284178cde245f308b7b71d10edf88bf33f3c3320))

## [0.3.0](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.2.1...v0.3.0) (2025-11-15)


### Features

* add support for 0.2 release series in metadata ([8c2f39a](https://github.com/mcanevet/cluster-api-provider-freebox/commit/8c2f39ab83a9331961ad7f667454ae334717b992))
* add support for 0.3 release series in metadata ([8ad9c1d](https://github.com/mcanevet/cluster-api-provider-freebox/commit/8ad9c1d38d549e1549dbac77285b4f37416a2a78))

## [0.2.1](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.2.0...v0.2.1) (2025-11-11)


### Bug Fixes

* split release workflows and require PAT for tag triggers ([619a3c3](https://github.com/mcanevet/cluster-api-provider-freebox/commit/619a3c3fc33138d3da6baf06b3fb20a452351c17))

## [0.2.0](https://github.com/mcanevet/cluster-api-provider-freebox/compare/v0.1.0...v0.2.0) (2025-11-11)


### Features

* add automated release artifact generation ([8d741f3](https://github.com/mcanevet/cluster-api-provider-freebox/commit/8d741f3f5be2a030e923b462eed82f9e85d398e2))

## 0.1.0 (2025-11-11)


### Features

* add CAPI v1beta2 contract label to CRDs ([14333ed](https://github.com/mcanevet/cluster-api-provider-freebox/commit/14333ed98422eca078e7058cd7d212274f1d21b5))
* add FreeboxMachineTemplate CRD and E2E tests ([e940540](https://github.com/mcanevet/cluster-api-provider-freebox/commit/e940540f76dea5dda44f0dcb27329aa2e866d974))
* add IP address population and disk type detection ([a51c69b](https://github.com/mcanevet/cluster-api-provider-freebox/commit/a51c69b3b1d5e2669405a6532ca84574345df093))
* add v1beta2 contract requirements ([1f0fbce](https://github.com/mcanevet/cluster-api-provider-freebox/commit/1f0fbcefc32c1174f9367b57b4b5e3ccc7284ed4))
* add v1beta2 contract requirements ([92bc6fd](https://github.com/mcanevet/cluster-api-provider-freebox/commit/92bc6fdbad86f7306d10e4aa936029a4e174d425))
* configure test-e2e ([8e82a0e](https://github.com/mcanevet/cluster-api-provider-freebox/commit/8e82a0e5e6f866cdf8469ad73f575440a2510419))
* create VM ([cbf105a](https://github.com/mcanevet/cluster-api-provider-freebox/commit/cbf105a5632a78586ff49f62c998f88a4fb8a057))
* delete VM on resource deletion ([be8da91](https://github.com/mcanevet/cluster-api-provider-freebox/commit/be8da91fb11bdc420a0faca3d61f2455e885a2b0))
* enable controller to read clusters and machines ([4b0ea74](https://github.com/mcanevet/cluster-api-provider-freebox/commit/4b0ea74711a5a0623402bfff0f77d51b6ffa5d1e))
* extract file when needed ([28753da](https://github.com/mcanevet/cluster-api-provider-freebox/commit/28753daf550fb8b9a8ff438c3be65050a382ef3d))
* huge improvements ([7df59df](https://github.com/mcanevet/cluster-api-provider-freebox/commit/7df59dfbd032d45cd1dc0ebdd6ee24b5430cac41))
* implement CAPI v1beta2 Ready condition lifecycle tracking ([6046f8c](https://github.com/mcanevet/cluster-api-provider-freebox/commit/6046f8c5e527e282f3f0c5c63142aff9cff9b231))
* initial freebox infrastructure provider release ([358fe0f](https://github.com/mcanevet/cluster-api-provider-freebox/commit/358fe0f37c69f8748ac23573c225f8ae3602d8ad))
* remove hardcoding ([a09cd23](https://github.com/mcanevet/cluster-api-provider-freebox/commit/a09cd238554ff908449cd222582edf9f1f2f6a3d))
* resize disk ([15bdf2e](https://github.com/mcanevet/cluster-api-provider-freebox/commit/15bdf2eeb5902578b118dcccffacf4566739a5e1))
* set providerID in spec to comply with CAPI v1beta2 contract ([e6e9ddf](https://github.com/mcanevet/cluster-api-provider-freebox/commit/e6e9ddf12268eec7424ab56d1e53f12aeaad718f))
* test with Talos image ([40ba7ec](https://github.com/mcanevet/cluster-api-provider-freebox/commit/40ba7ec5279357f59bde04c7b1f3f2f69c1d3d0f))
* working download ([b12adcb](https://github.com/mcanevet/cluster-api-provider-freebox/commit/b12adcb973df189f45e250c766441490fa26dc48))


### Bug Fixes

* **ci:** fix go-lint issues ([d327267](https://github.com/mcanevet/cluster-api-provider-freebox/commit/d3272670eb32db21076a475c9ce803a999e1537b))
* **ci:** rename release-please config file to default name ([0ccaaba](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0ccaaba8cc0019d71e5d39e20a12a9e933dba3e2))
* **ci:** rename release-please manifest file to default name ([7bfe4fa](https://github.com/mcanevet/cluster-api-provider-freebox/commit/7bfe4fa13f1cce5a88653f3d1fcfa2ba476293d5))
* commit missing change ([7d0ea63](https://github.com/mcanevet/cluster-api-provider-freebox/commit/7d0ea63ecba44d0c34dbac9404a7ff41518130e9))
* mitigate dependency on controller-runtime ([bf5cc18](https://github.com/mcanevet/cluster-api-provider-freebox/commit/bf5cc18109bce7618f4e9be42aefcde8838b5458))
* properly manage VM lifecycle ([914a053](https://github.com/mcanevet/cluster-api-provider-freebox/commit/914a053b2d139a763833bc215863e3aef21c1063))
* remove associated disk and bios files ([0c8bbf6](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0c8bbf6c568c94a81a2512d8e102960fa75ebb26))
* use arm64image ([5be4f77](https://github.com/mcanevet/cluster-api-provider-freebox/commit/5be4f77c1bc1d04132f7d022f60448ba6f56740d))
* use debian generic image to have cloud-init ([1a3507e](https://github.com/mcanevet/cluster-api-provider-freebox/commit/1a3507e6ccaefbf48f6cb2f0a5db13e364d6b6d5))
* use proper disk type ([2b73f62](https://github.com/mcanevet/cluster-api-provider-freebox/commit/2b73f625a6569228d39c338ec83529a755cedb9f))
* use the VM name as image name ([37bc092](https://github.com/mcanevet/cluster-api-provider-freebox/commit/37bc0921cd6eb86c2c9907d263e3ea6fb138f35e))


### Miscellaneous Chores

* release 0.1.0 ([4e99e1f](https://github.com/mcanevet/cluster-api-provider-freebox/commit/4e99e1f7a69ae276c59983685e424e4388acbe52))
