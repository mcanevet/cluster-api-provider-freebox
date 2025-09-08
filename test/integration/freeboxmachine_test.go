package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	freeclient "github.com/nikolalohinski/free-go/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/mcanevet/cluster-api-provider-freebox/api/v1alpha1"
)

// Test constants
const (
	integrationTestsEnvVar = "INTEGRATION_TESTS"
	enabledValue           = "true"
	latestVersion          = "latest"
)

// TestFreeboxMachineVMCreation tests the complete VM lifecycle through the FreeboxMachine controller
func TestFreeboxMachineVMCreation(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv(integrationTestsEnvVar) != enabledValue {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=true to run.")
	}

	// Create a test FreeboxMachine
	machine := &infrastructurev1alpha1.FreeboxMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm-" + fmt.Sprintf("%d", time.Now().Unix()),
			Namespace: "default",
		},
		Spec: infrastructurev1alpha1.FreeboxMachineSpec{
			VCPUs:    1,
			Memory:   2048, // 2GB in MB
			DiskSize: 20,   // 20GB
		},
	}

	t.Run("CreateVM", func(t *testing.T) {
		// This test will fail initially (Red phase) until we implement the controller
		// The controller should:
		// 1. Create a VM on the Freebox with the specified resources
		// 2. Update the machine status with provider ID
		// 3. Set the machine phase to "Running"
		// 4. Update machine addresses with VM IP

		// TODO: This will be implemented when we have the controller
		// For now, let's test the basic VM creation via direct API calls
		t.Logf("Created test FreeboxMachine: %s", machine.Name)
		t.Skip("Controller implementation needed - this is the RED phase of TDD")
	})

	t.Run("DeleteVM", func(t *testing.T) {
		// This test will verify VM deletion
		t.Skip("Controller implementation needed - this is the RED phase of TDD")
	})

	t.Run("VMStatusUpdates", func(t *testing.T) {
		// This test will verify status updates
		t.Skip("Controller implementation needed - this is the RED phase of TDD")
	})
}

// TestFreeboxMachineDirectVMOperations tests VM operations directly via Freebox API
// This validates our understanding of the Freebox API before implementing the controller
func TestFreeboxMachineDirectVMOperations(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv(integrationTestsEnvVar) != enabledValue {
		t.Skip("Skipping integration test. Set INTEGRATION_TESTS=true to run.")
	}

	ctx := context.Background()

	// Initialize Freebox client using the same pattern as existing tests
	endpoint := os.Getenv("FREEBOX_ENDPOINT")
	require.NotEmpty(t, endpoint, "FREEBOX_ENDPOINT must be set")

	version := os.Getenv("FREEBOX_VERSION")
	if version == "" {
		version = latestVersion
	}

	appID := os.Getenv("FREEBOX_APP_ID")
	require.NotEmpty(t, appID, "FREEBOX_APP_ID must be set")

	token := os.Getenv("FREEBOX_TOKEN")
	require.NotEmpty(t, token, "FREEBOX_TOKEN must be set")

	// Create Freebox client
	fbClient, err := freeclient.New(endpoint, version)
	require.NoError(t, err, "Failed to create Freebox client")

	// Configure authentication
	fbClient = fbClient.WithAppID(appID).WithPrivateToken(token)

	// Test login
	_, err = fbClient.Login(ctx)
	require.NoError(t, err, "Failed to login to Freebox")

	t.Run("ListExistingVMs", func(t *testing.T) {
		vms, err := fbClient.ListVirtualMachines(ctx)
		require.NoError(t, err)

		t.Logf("Found %d virtual machines", len(vms))
		for _, vm := range vms {
			t.Logf("VM: ID=%d, Name=%s, Status=%s, CPUs=%d, Memory=%d",
				vm.ID, vm.Name, vm.Status, vm.VCPUs, vm.Memory)
		}
	})

	t.Run("ValidateVMResourceLimits", func(t *testing.T) {
		systemInfo, err := fbClient.GetVirtualMachineInfo(ctx)
		require.NoError(t, err)

		t.Logf("VM System Limits: CPUs=%d (used: %d), Memory=%d MB (used: %d)",
			systemInfo.TotalCPUs, systemInfo.UsedCPUs,
			systemInfo.TotalMemory, systemInfo.UsedMemory)

		// Verify we have enough resources for test VMs
		assert.Greater(t, int(systemInfo.TotalCPUs-systemInfo.UsedCPUs), 0, "Need at least 1 CPU available")
		assert.Greater(t, int(systemInfo.TotalMemory-systemInfo.UsedMemory), 1024, "Need at least 1GB RAM available")
	})

	// This test validates we understand the VM creation API
	// We'll use this knowledge to implement the controller
	t.Run("UnderstandVMCreationAPI", func(t *testing.T) {
		// NOTE: This is a dry-run test to understand the API structure
		// We won't actually create VMs here to avoid cluttering the Freebox

		t.Log("VM Creation would use these parameters:")
		t.Log("- Name: unique identifier for the VM")
		t.Log("- CPUs: number of virtual CPUs (1-3 based on hardware)")
		t.Log("- Memory: RAM in MB (max 15360 MB available)")
		t.Log("- Disk: storage configuration")
		t.Log("- Network: default network configuration")

		// TODO: Once we implement the controller, we'll have actual VM creation tests
		// For now, this documents our API understanding
	})
}
