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
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	freeboxclient "github.com/nikolalohinski/free-go/client"
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

	// freeboxClient is the Freebox API client for E2E tests
	freeboxClient freeboxclient.Client
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

	By("Overriding config with environment variables if set")
	// Allow environment variables to override config file values
	if envVal := os.Getenv("TEST_IMAGE_URL"); envVal != "" {
		e2eConfig.Variables["TEST_IMAGE_URL"] = envVal
	}
	if envVal := os.Getenv("FREEBOX_ENDPOINT"); envVal != "" {
		e2eConfig.Variables["FREEBOX_ENDPOINT"] = envVal
	}
	if envVal := os.Getenv("FREEBOX_APP_ID"); envVal != "" {
		e2eConfig.Variables["FREEBOX_APP_ID"] = envVal
	}
	if envVal := os.Getenv("FREEBOX_TOKEN"); envVal != "" {
		e2eConfig.Variables["FREEBOX_TOKEN"] = envVal
	}
	if envVal := os.Getenv("FREEBOX_VERSION"); envVal != "" {
		e2eConfig.Variables["FREEBOX_VERSION"] = envVal
	}

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

	By("Initializing Freebox client for E2E tests")
	freeboxEndpoint := e2eConfig.Variables["FREEBOX_ENDPOINT"]
	if freeboxEndpoint == "" {
		freeboxEndpoint = "http://mafreebox.freebox.fr"
	}

	freeboxVersion := e2eConfig.Variables["FREEBOX_VERSION"]
	if freeboxVersion == "" {
		freeboxVersion = "latest"
	}

	freeboxClient, err = freeboxclient.New(freeboxEndpoint, freeboxVersion)
	Expect(err).ToNot(HaveOccurred(), "Failed to create Freebox client")

	freeboxAppID := e2eConfig.Variables["FREEBOX_APP_ID"]
	Expect(freeboxAppID).ToNot(BeEmpty(), "FREEBOX_APP_ID must be set")
	freeboxClient.WithAppID(freeboxAppID)

	freeboxToken := e2eConfig.Variables["FREEBOX_TOKEN"]
	Expect(freeboxToken).ToNot(BeEmpty(), "FREEBOX_TOKEN must be set")
	freeboxClient.WithPrivateToken(freeboxToken)

	By("Getting Freebox session token for API calls")
	// Get a session token for our direct API calls since free-go doesn't expose all endpoints
	sessionToken, err := getFreeboxSessionToken(freeboxEndpoint, freeboxVersion, freeboxAppID, freeboxToken)
	Expect(err).ToNot(HaveOccurred(), "failed to get session token")

	By("Fetching Freebox download directory from Freebox download config")
	// Query the Freebox API to get the default download directory and require it.
	// This is a direct HTTP call since free-go doesn't expose /downloads/config/ yet.
	freeboxDownloadDir, err := getFreeboxDownloadDir(freeboxEndpoint, freeboxVersion, sessionToken)
	Expect(err).ToNot(HaveOccurred(), "failed to get download_dir from Freebox /downloads/config/")

	// Use the download_dir from the Freebox API unconditionally.
	e2eConfig.Variables["FREEBOX_DOWNLOAD_DIR"] = freeboxDownloadDir
	GinkgoLogr.Info("Using Freebox download directory (from Freebox /downloads/config)", "path", freeboxDownloadDir)

	By("Fetching VM storage path from Freebox system config")
	// Query the Freebox API to get the VM storage path and require it.
	// This is a direct HTTP call since free-go doesn't expose /system/ yet.
	vmStoragePath, err := getVMStoragePath(freeboxEndpoint, freeboxVersion, sessionToken)
	Expect(err).ToNot(HaveOccurred(), "failed to get user_main_storage from Freebox /system/")

	// Use the VM storage path from the Freebox API unconditionally.
	e2eConfig.Variables["VM_STORAGE_PATH"] = vmStoragePath
	GinkgoLogr.Info("Using VM storage path (from Freebox /system/ user_main_storage)", "path", vmStoragePath)

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

// getFreeboxDownloadDir queries the Freebox API to get the default download directory.
// This is a direct HTTP call since the free-go library doesn't expose the
// /downloads/config/ endpoint yet. Consider contributing this to free-go in the future.
func getFreeboxDownloadDir(endpoint, version, sessionToken string) (string, error) {
	// Construct the URL for the downloads config endpoint
	configURL := fmt.Sprintf("%s/api/%s/downloads/config/", endpoint, version)

	// Create HTTP request
	req, err := http.NewRequest("GET", configURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header with session token
	req.Header.Set("X-Fbx-App-Auth", sessionToken)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var result struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			DownloadDir string `json:"download_dir"` // Base64 encoded path
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !result.Success {
		if result.ErrorCode != "" || result.Msg != "" {
			return "", fmt.Errorf("API call failed: error_code=%s, msg=%s", result.ErrorCode, result.Msg)
		}
		return "", fmt.Errorf("API call was not successful (no error details provided)")
	}

	// Decode base64 download_dir
	decodedBytes, err := base64.StdEncoding.DecodeString(result.Result.DownloadDir)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 download_dir: %w", err)
	}

	downloadDir := string(decodedBytes)
	if downloadDir == "" {
		return "", fmt.Errorf("download_dir is empty after decoding")
	}

	return downloadDir, nil
}

// getVMStoragePath queries the Freebox API to get the VM storage path.
// This is a direct HTTP call since the free-go library doesn't expose the
// /system/ endpoint yet. Consider contributing this to free-go in the future.
func getVMStoragePath(endpoint, version, sessionToken string) (string, error) {
	// Construct the URL for the system endpoint
	systemURL := fmt.Sprintf("%s/api/%s/system/", endpoint, version)

	// Create HTTP request
	req, err := http.NewRequest("GET", systemURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header with session token
	req.Header.Set("X-Fbx-App-Auth", sessionToken)

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var result struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			UserMainStorage string `json:"user_main_storage"` // Plain string like "Disque 1", NOT base64 encoded
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !result.Success {
		if result.ErrorCode != "" || result.Msg != "" {
			return "", fmt.Errorf("API call failed: error_code=%s, msg=%s", result.ErrorCode, result.Msg)
		}
		return "", fmt.Errorf("API call was not successful (no error details provided)")
	}

	// Check if user_main_storage is empty
	if result.Result.UserMainStorage == "" {
		return "", fmt.Errorf("user_main_storage is empty in response")
	}

	// Note: user_main_storage is NOT base64 encoded, it's a plain string like "Disque 1"
	// So we use it directly without decoding
	mainStorage := result.Result.UserMainStorage
	if mainStorage == "" {
		return "", fmt.Errorf("user_main_storage is empty")
	}

	// The main storage is just a disk name like "Disque 1", we need to construct the full path
	// According to Freebox conventions, the path is /DiskName/
	vmStoragePath := "/" + mainStorage + "/VMs"

	return vmStoragePath, nil
}

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

// getFreeboxSessionToken creates a session token for direct API calls.
// This is needed because free-go doesn't expose some endpoints we need.
func getFreeboxSessionToken(endpoint, version, appID, privateToken string) (string, error) {
	// Step 1: Get the login challenge
	challengeURL := fmt.Sprintf("%s/api/%s/login", endpoint, version)
	resp, err := http.Get(challengeURL)
	if err != nil {
		return "", fmt.Errorf("failed to get login challenge: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read challenge response: %w", err)
	}

	var challengeResult struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			Challenge string `json:"challenge"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &challengeResult); err != nil {
		return "", fmt.Errorf("failed to parse challenge response: %w", err)
	}

	if !challengeResult.Success {
		if challengeResult.ErrorCode != "" || challengeResult.Msg != "" {
			return "", fmt.Errorf("challenge request failed: error_code=%s, msg=%s", challengeResult.ErrorCode, challengeResult.Msg)
		}
		return "", fmt.Errorf("challenge request was not successful")
	}

	// Step 2: Compute the password (HMAC-SHA1 of challenge with private token)
	//nolint:gosec // SHA1 is required by Freebox API
	h := hmac.New(sha1.New, []byte(privateToken))
	h.Write([]byte(challengeResult.Result.Challenge))
	password := hex.EncodeToString(h.Sum(nil))

	// Step 3: Open a session
	sessionURL := fmt.Sprintf("%s/api/%s/login/session", endpoint, version)
	sessionPayload := fmt.Sprintf(`{"app_id":"%s","password":"%s"}`, appID, password)

	sessionResp, err := http.Post(sessionURL, "application/json", strings.NewReader(sessionPayload))
	if err != nil {
		return "", fmt.Errorf("failed to open session: %w", err)
	}
	defer sessionResp.Body.Close()

	sessionBody, err := io.ReadAll(sessionResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read session response: %w", err)
	}

	var sessionResult struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"error_code,omitempty"`
		Msg       string `json:"msg,omitempty"`
		Result    struct {
			SessionToken string `json:"session_token"`
		} `json:"result"`
	}

	if err := json.Unmarshal(sessionBody, &sessionResult); err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	if !sessionResult.Success {
		if sessionResult.ErrorCode != "" || sessionResult.Msg != "" {
			return "", fmt.Errorf("session request failed: error_code=%s, msg=%s", sessionResult.ErrorCode, sessionResult.Msg)
		}
		return "", fmt.Errorf("session request was not successful")
	}

	return sessionResult.Result.SessionToken, nil
}
