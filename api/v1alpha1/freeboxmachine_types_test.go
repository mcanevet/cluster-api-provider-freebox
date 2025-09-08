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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFreeboxMachine_GetVCPUs(t *testing.T) {
	tests := []struct {
		name     string
		machine  *FreeboxMachine
		expected int
	}{
		{
			name: "returns specified VCPUs",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					VCPUs: 3,
				},
			},
			expected: 3,
		},
		{
			name: "returns default when VCPUs is 0",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					VCPUs: 0,
				},
			},
			expected: 2,
		},
		{
			name: "returns default when VCPUs is not set",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.machine.GetVCPUs()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFreeboxMachine_GetMemory(t *testing.T) {
	tests := []struct {
		name     string
		machine  *FreeboxMachine
		expected int
	}{
		{
			name: "returns specified memory",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					Memory: 4096,
				},
			},
			expected: 4096,
		},
		{
			name: "returns default when memory is 0",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					Memory: 0,
				},
			},
			expected: 2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.machine.GetMemory()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFreeboxMachine_GetDiskSize(t *testing.T) {
	tests := []struct {
		name     string
		machine  *FreeboxMachine
		expected int
	}{
		{
			name: "returns specified disk size",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					DiskSize: 50,
				},
			},
			expected: 50,
		},
		{
			name: "returns default when disk size is 0",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					DiskSize: 0,
				},
			},
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.machine.GetDiskSize()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFreeboxMachine_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		machine  *FreeboxMachine
		expected bool
	}{
		{
			name: "returns true when ready",
			machine: &FreeboxMachine{
				Status: FreeboxMachineStatus{
					Ready: true,
				},
			},
			expected: true,
		},
		{
			name: "returns false when not ready",
			machine: &FreeboxMachine{
				Status: FreeboxMachineStatus{
					Ready: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.machine.IsReady()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFreeboxMachine_GetProviderID(t *testing.T) {
	statusProviderID := "status-provider-id"
	specProviderID := "spec-provider-id"

	tests := []struct {
		name     string
		machine  *FreeboxMachine
		expected string
	}{
		{
			name: "returns status provider ID when set",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					ProviderID: &specProviderID,
				},
				Status: FreeboxMachineStatus{
					ProviderID: &statusProviderID,
				},
			},
			expected: statusProviderID,
		},
		{
			name: "returns spec provider ID when status not set",
			machine: &FreeboxMachine{
				Spec: FreeboxMachineSpec{
					ProviderID: &specProviderID,
				},
				Status: FreeboxMachineStatus{},
			},
			expected: specProviderID,
		},
		{
			name: "returns empty string when neither set",
			machine: &FreeboxMachine{
				Spec:   FreeboxMachineSpec{},
				Status: FreeboxMachineStatus{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.machine.GetProviderID()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFreeboxMachine_Creation(t *testing.T) {
	machine := &FreeboxMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine",
			Namespace: "default",
		},
		Spec: FreeboxMachineSpec{
			VCPUs:    2,
			Memory:   4096,
			DiskSize: 30,
		},
	}

	assert.Equal(t, "test-machine", machine.Name)
	assert.Equal(t, "default", machine.Namespace)
	assert.Equal(t, 2, machine.GetVCPUs())
	assert.Equal(t, 4096, machine.GetMemory())
	assert.Equal(t, 30, machine.GetDiskSize())
	assert.False(t, machine.IsReady())
	assert.Empty(t, machine.GetProviderID())
}
