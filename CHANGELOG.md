# Changelog

## 1.0.0 (2025-11-11)


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
* commit missing change ([7d0ea63](https://github.com/mcanevet/cluster-api-provider-freebox/commit/7d0ea63ecba44d0c34dbac9404a7ff41518130e9))
* mitigate dependency on controller-runtime ([bf5cc18](https://github.com/mcanevet/cluster-api-provider-freebox/commit/bf5cc18109bce7618f4e9be42aefcde8838b5458))
* properly manage VM lifecycle ([914a053](https://github.com/mcanevet/cluster-api-provider-freebox/commit/914a053b2d139a763833bc215863e3aef21c1063))
* remove associated disk and bios files ([0c8bbf6](https://github.com/mcanevet/cluster-api-provider-freebox/commit/0c8bbf6c568c94a81a2512d8e102960fa75ebb26))
* use arm64image ([5be4f77](https://github.com/mcanevet/cluster-api-provider-freebox/commit/5be4f77c1bc1d04132f7d022f60448ba6f56740d))
* use debian generic image to have cloud-init ([1a3507e](https://github.com/mcanevet/cluster-api-provider-freebox/commit/1a3507e6ccaefbf48f6cb2f0a5db13e364d6b6d5))
* use proper disk type ([2b73f62](https://github.com/mcanevet/cluster-api-provider-freebox/commit/2b73f625a6569228d39c338ec83529a755cedb9f))
* use the VM name as image name ([37bc092](https://github.com/mcanevet/cluster-api-provider-freebox/commit/37bc0921cd6eb86c2c9907d263e3ea6fb138f35e))
