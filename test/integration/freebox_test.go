package integration

import (
	"context"
	"os"
	"testing"

	"github.com/nikolalohinski/free-go/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFreeboxConnection tests basic connectivity to the Freebox using environment variables
func TestFreeboxConnection(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=true to run.")
	}

	ctx := context.Background()

	// Get configuration from environment variables (set via mise)
	endpoint := os.Getenv("FREEBOX_ENDPOINT")
	require.NotEmpty(t, endpoint, "FREEBOX_ENDPOINT must be set")

	version := os.Getenv("FREEBOX_VERSION")
	if version == "" {
		version = "latest"
	}

	appID := os.Getenv("FREEBOX_APP_ID")
	require.NotEmpty(t, appID, "FREEBOX_APP_ID must be set")

	token := os.Getenv("FREEBOX_TOKEN")
	require.NotEmpty(t, token, "FREEBOX_TOKEN must be set")

	// Create Freebox client
	freeboxClient, err := client.New(endpoint, version)
	require.NoError(t, err, "Failed to create Freebox client")

	// Configure authentication
	freeboxClient = freeboxClient.WithAppID(appID).WithPrivateToken(token)

	// Test login
	permissions, err := freeboxClient.Login(ctx)
	require.NoError(t, err, "Failed to login to Freebox")
	assert.NotNil(t, permissions, "Permissions should not be nil")

	t.Logf("Successfully connected to Freebox with permissions: %+v", permissions)
}

// TestFreeboxVMCapabilities tests VM-related capabilities
func TestFreeboxVMCapabilities(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=true to run.")
	}

	ctx := context.Background()

	// Setup client (reuse from previous test)
	freeboxClient := setupFreeboxClient(t)

	// Login
	_, err := freeboxClient.Login(ctx)
	require.NoError(t, err, "Failed to login to Freebox")

	// Test VM system info
	sysInfo, err := freeboxClient.GetVirtualMachineInfo(ctx)
	require.NoError(t, err, "Failed to get VM system info")
	assert.NotNil(t, sysInfo, "VM system info should not be nil")

	t.Logf("VM System Info: %+v", sysInfo)

	// Test list virtual machines
	vms, err := freeboxClient.ListVirtualMachines(ctx)
	require.NoError(t, err, "Failed to list virtual machines")
	assert.NotNil(t, vms, "VM list should not be nil")

	t.Logf("Found %d virtual machines", len(vms))
}

// setupFreeboxClient is a helper function to create a Freebox client
func setupFreeboxClient(t *testing.T) client.Client {
	endpoint := os.Getenv("FREEBOX_ENDPOINT")
	require.NotEmpty(t, endpoint, "FREEBOX_ENDPOINT must be set")

	version := os.Getenv("FREEBOX_VERSION")
	if version == "" {
		version = "latest"
	}

	appID := os.Getenv("FREEBOX_APP_ID")
	require.NotEmpty(t, appID, "FREEBOX_APP_ID must be set")

	token := os.Getenv("FREEBOX_TOKEN")
	require.NotEmpty(t, token, "FREEBOX_TOKEN must be set")

	freeboxClient, err := client.New(endpoint, version)
	require.NoError(t, err, "Failed to create Freebox client")

	// Configure authentication
	freeboxClient = freeboxClient.WithAppID(appID).WithPrivateToken(token)

	return freeboxClient
}
