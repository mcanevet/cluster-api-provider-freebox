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
	// VCPUs specifies the number of virtual CPUs for the VM
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +optional
	VCPUs int `json:"vcpus,omitempty"`

	// Memory specifies the amount of memory for the VM in MB
	// +kubebuilder:validation:Minimum=512
	// +kubebuilder:validation:Maximum=15360
	// +optional
	Memory int `json:"memory,omitempty"`

	// DiskSize specifies the disk size for the VM in GB
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	DiskSize int `json:"diskSize,omitempty"`

	// CloudInit contains cloud-init configuration for the VM
	// +optional
	CloudInit *CloudInitSpec `json:"cloudInit,omitempty"`

	// ProviderID is the unique identifier for the VM on the Freebox
	// This field is set by the controller and should not be modified by users
	// +optional
	ProviderID *string `json:"providerID,omitempty"`
}

// CloudInitSpec defines cloud-init configuration
type CloudInitSpec struct {
	// UserData contains the user data for cloud-init
	// +optional
	UserData string `json:"userData,omitempty"`

	// MetaData contains the meta data for cloud-init
	// +optional
	MetaData string `json:"metaData,omitempty"`
}

// FreeboxMachineStatus defines the observed state of FreeboxMachine.
type FreeboxMachineStatus struct {
	// Ready indicates whether the machine is ready to be used
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Addresses contains the VM's IP addresses
	// +optional
	Addresses []string `json:"addresses,omitempty"`

	// ProviderID is the unique identifier for the VM on the Freebox
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// VMState represents the current state of the VM (running, stopped, etc.)
	// +optional
	VMState string `json:"vmState,omitempty"`

	// ErrorMessage contains error information if the machine is in an error state
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`

	// conditions represent the current state of the FreeboxMachine resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the resource is fully functional and VM is running
	// - "VMCreated": the VM has been created on the Freebox
	// - "VMStarted": the VM has been started
	// - "InfrastructureReady": the infrastructure is ready for use
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
	SchemeBuilder.Register(&FreeboxMachine{}, &FreeboxMachineList{})
}
