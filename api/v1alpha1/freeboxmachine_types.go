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
	// Name of the VM in the Freebox
	Name string `json:"name"`
	// Number of vCPUs
	CPU int `json:"cpu"`
	// Size of the RAM in MB
	Memory int `json:"memory"`
	// Image to use (ex: "debian-bullseye")
	// +optional
	Image string `json:"image,omitempty"`
}

// FreeboxMachineStatus defines the observed state of FreeboxMachine.
type FreeboxMachineStatus struct {
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

	// ID of the VM in the Freebox
	VMID string `json:"vmId,omitempty"`
	// State of the VM (running, stopped, error, etc.)
	State string `json:"state,omitempty"`
	// IP address of the VM if available
	IPAddress string `json:"ipAddress,omitempty"`
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
