//go:build e2e
// +build e2e

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

var (
	ctx = ctrl.SetupSignalHandler()

	// clusterctlConfigPath is the path to the clusterctl config file
	clusterctlConfigPath string

	// e2eConfig is the configuration for the e2e test
	e2eConfig *clusterctl.E2EConfig

	// clusterProxy is the proxy to the management cluster
	clusterProxy framework.ClusterProxy

	// clusterProvider is the bootstrap cluster provider
	clusterProvider bootstrap.ClusterProvider

	// artifactFolder is where test artifacts should be stored
	artifactFolder string

	// skipCleanup prevents cleanup of test resources
	skipCleanup bool
)

// TestE2E runs the end-to-end (e2e) test suite for the Freebox provider.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	ctrl.SetLogger(GinkgoLogr)
	RunSpecs(t, "Freebox Provider E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Run only once before all test processes
	Expect(ctx).NotTo(BeNil(), "ctx is required for e2e tests")

	By("Loading e2e test configuration")
	configPath := os.Getenv("E2E_CONF_FILE")
	if configPath == "" {
		configPath = filepath.Join("..", "..", "test", "e2e", "config", "freebox.yaml")
	}
	// Get absolute path to config file
	absConfigPath, err := filepath.Abs(configPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to get absolute path for config file")

	e2eConfig = clusterctl.LoadE2EConfig(ctx, clusterctl.LoadE2EConfigInput{ConfigPath: absConfigPath})
	Expect(e2eConfig).ToNot(BeNil(), "Failed to load E2E config from %s", absConfigPath)

	By("Setting up artifact folder")
	artifactFolder = os.Getenv("ARTIFACTS")
	if artifactFolder == "" {
		artifactFolder = filepath.Join("..", "..", "_artifacts")
	}
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Failed to create artifact folder %s", artifactFolder)

	By("Creating the bootstrap cluster")
	// Create a kind cluster to use as the management cluster
	clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
		Name:               e2eConfig.ManagementClusterName,
		KubernetesVersion:  e2eConfig.Variables["KUBERNETES_VERSION_MANAGEMENT"],
		RequiresDockerSock: false,
		Images:             e2eConfig.Images,
		LogFolder:          artifactFolder,
	})
	Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

	By("Setting up the cluster proxy")
	clusterProxy = framework.NewClusterProxy("freebox-e2e", clusterProvider.GetKubeconfigPath(), initScheme())
	Expect(clusterProxy).ToNot(BeNil())

	By("Creating clusterctl local repository")
	repositoryFolder, err := filepath.Abs(filepath.Join(artifactFolder, "repository"))
	Expect(err).ToNot(HaveOccurred(), "Failed to get absolute path for repository folder")
	clusterctlConfigPath = createClusterctlLocalRepository(e2eConfig, repositoryFolder, absConfigPath)

	By("Initializing the management cluster with providers")
	clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
		ClusterProxy:            clusterProxy,
		ClusterctlConfigPath:    clusterctlConfigPath,
		InfrastructureProviders: e2eConfig.InfrastructureProviders(),
		LogFolder:               filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
	}, e2eConfig.GetIntervals(clusterProxy.GetName(), "wait-controllers")...)

	return nil
}, func(data []byte) {
	// Run before each test process
})

var _ = SynchronizedAfterSuite(func() {
	// Run after each test process
}, func() {
	// Run only once after all test processes
	By("Tearing down the management cluster")
	if !skipCleanup {
		if clusterProxy != nil {
			clusterProxy.Dispose(ctx)
		}
		if clusterProvider != nil {
			clusterProvider.Dispose(ctx)
		}
	}
})

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(infrastructurev1alpha1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

// createClusterctlLocalRepository creates a local clusterctl repository.
func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string, configPath string) string {
	// Convert all relative paths in the config to absolute paths
	absRepositoryFolder, err := filepath.Abs(repositoryFolder)
	Expect(err).ToNot(HaveOccurred(), "Failed to get absolute path for repository folder")

	// Use the directory containing the config file as the base path for relative paths
	configDir := filepath.Dir(configPath)
	config.AbsPaths(configDir)

	return clusterctl.CreateRepository(ctx, clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: absRepositoryFolder,
	})
}
