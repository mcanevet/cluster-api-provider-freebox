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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FreeboxMachineSpec defines the desired state of FreeboxMachine
type FreeboxMachineSpec struct {
	// providerID must match the provider ID as seen on the node object corresponding to this machine.
	// For Kubernetes Nodes running on the Freebox provider, this value is set by the corresponding CPI component
	// and it has the format freebox:////<vm-name>.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	ProviderID string `json:"providerID,omitempty"`

	// Name of the VM in the Freebox
	Name string `json:"name"`
	// Number of vCPUs
	// +kubebuilder:validation:Minimum=1
	VCPUs int64 `json:"vcpus"` // e.g. 2
	// Size of the RAM in MB
	// +kubebuilder:validation:Minimum=1
	MemoryMB int64 `json:"memoryMB"` // e.g. 2048 for 2GB
	// Size of the disk in MB
	DiskSizeBytes int64 `json:"diskSizeBytes"`
	// Image to use (ex: "debian-bullseye")
	ImageURL string `json:"imageURL"`
}

// FreeboxMachineStatus defines the observed state of FreeboxMachine.
type FreeboxMachineStatus struct {
	// initialization provides observations of the FreeboxMachine initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization FreeboxMachineInitializationStatus `json:"initialization,omitempty,omitzero"`

	// conditions represent the current state of the FreeboxMachine resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// VMID stores the ID of the created Freebox virtual machine
	// so it can be deleted when the FreeboxMachine is deleted.
	VMID int64 `json:"vmID,omitempty"`

	// DiskPath stores the path to the VM disk file
	// so it can be deleted when the FreeboxMachine is deleted.
	DiskPath string `json:"diskPath,omitempty"`
}

// FreeboxMachineInitializationStatus provides observations of the FreeboxMachine initialization process.
// +kubebuilder:validation:MinProperties=1
type FreeboxMachineInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the Machine's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FreeboxMachine is the Schema for the freeboxmachines API
type FreeboxMachine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of FreeboxMachine
	// +required
	Spec FreeboxMachineSpec `json:"spec"`

	// status defines the observed state of FreeboxMachine
	// +optional
	Status FreeboxMachineStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FreeboxMachineList contains a list of FreeboxMachine
type FreeboxMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FreeboxMachine `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &FreeboxMachine{}, &FreeboxMachineList{})
}
