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

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	freeboxclient "github.com/nikolalohinski/free-go/client"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
	"github.com/mcanevet/cluster-api-provider-freebox/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "9ecca9fd.cluster.x-k8s.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	freeboxEndpoint := os.Getenv("FREEBOX_ENDPOINT")
	if freeboxEndpoint == "" {
		freeboxEndpoint = "http://mafreebox.freebox.fr"
	}

	freeboxVersion := os.Getenv("FREEBOX_VERSION")
	if freeboxVersion == "" {
		freeboxVersion = "latest"
	}

	fbClient, err := freeboxclient.New(freeboxEndpoint, freeboxVersion)
	if err != nil {
		setupLog.Error(err, "unable to create freebox client")
		os.Exit(1)
	}

	freeboxAppID := os.Getenv("FREEBOX_APP_ID")
	if freeboxAppID == "" {
		setupLog.Error(err, "FREEBOX_APP_ID undefined")
		os.Exit(1)
	}
	fbClient.WithAppID(freeboxAppID)

	freeboxToken := os.Getenv("FREEBOX_TOKEN")
	if freeboxToken == "" {
		setupLog.Error(err, "FREEBOX_TOKEN undefined")
		os.Exit(1)
	}
	fbClient.WithPrivateToken(freeboxToken)

	setupLog.Info("Freebox client created successfully")

	// Login to establish a session (this validates credentials work)
	ctx := context.Background()
	permissions, err := fbClient.Login(ctx)
	if err != nil {
		setupLog.Error(err, "unable to login to Freebox")
		os.Exit(1)
	}
	setupLog.Info("Logged in to Freebox successfully", "permissions", permissions)

	// Get a session token for our direct API calls
	// Since free-go doesn't expose /downloads/config/ and /system/ endpoints,
	// we need to make direct HTTP calls with our own session
	sessionToken, err := getFreeboxSessionToken(freeboxEndpoint, freeboxVersion, freeboxAppID, freeboxToken)
	if err != nil {
		setupLog.Error(err, "unable to get session token for API calls")
		os.Exit(1)
	}

	// Fetch Freebox download directory from Freebox download config
	freeboxDownloadDir, err := getFreeboxDownloadDir(freeboxEndpoint, freeboxVersion, sessionToken)
	if err != nil {
		setupLog.Error(err, "unable to fetch download_dir from Freebox /downloads/config/")
		os.Exit(1)
	}
	setupLog.Info("Using Freebox download directory from /downloads/config", "path", freeboxDownloadDir)

	// Fetch VM storage path from Freebox system config
	vmStoragePath, err := getVMStoragePath(freeboxEndpoint, freeboxVersion, sessionToken)
	if err != nil {
		setupLog.Error(err, "unable to fetch user_main_storage from Freebox /system/")
		os.Exit(1)
	}
	setupLog.Info("Using VM storage path from /system/ user_main_storage", "path", vmStoragePath)

	// // TODO: remove this
	// ctx := context.Background()

	// vms, err := client.ListVirtualMachines(ctx)
	// if err != nil {
	// 	setupLog.Error(err, "Can not list VMs")
	// 	os.Exit(1)
	// }

	// if len(vms) == 0 {
	// 	setupLog.Info("No VMs found")
	// } else {
	// 	for _, vm := range vms {
	// 		setupLog.Info("VM found", "ID", vm.ID, "Name", vm.Name, "Status", vm.Status)
	// 	}
	// }
	// // END TODO

	if err := (&controller.FreeboxClusterReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		FreeboxClient: fbClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FreeboxCluster")
		os.Exit(1)
	}
	if err := (&controller.FreeboxMachineReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		FreeboxClient:      fbClient,
		FreeboxDownloadDir: freeboxDownloadDir,
		VMStoragePath:      vmStoragePath,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FreeboxMachine")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

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
	defer func() {
		_ = resp.Body.Close()
	}()

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
	defer func() {
		_ = resp.Body.Close()
	}()

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

// getFreeboxSessionToken creates a session token for direct API calls.
// This is needed because free-go doesn't expose some endpoints we need.
func getFreeboxSessionToken(endpoint, version, appID, privateToken string) (string, error) {
	// Step 1: Get the login challenge
	challengeURL := fmt.Sprintf("%s/api/%s/login", endpoint, version)
	resp, err := http.Get(challengeURL)
	if err != nil {
		return "", fmt.Errorf("failed to get login challenge: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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
			return "", fmt.Errorf(
				"challenge request failed: error_code=%s, msg=%s",
				challengeResult.ErrorCode,
				challengeResult.Msg,
			)
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
	defer func() {
		_ = sessionResp.Body.Close()
	}()

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
