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

// FreeboxClusterSpec defines the desired state of FreeboxCluster
type FreeboxClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane
	// For single VM setup, this will be the VM's IP address
	// +optional
	ControlPlaneEndpoint APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint
type APIEndpoint struct {
	// Host is the hostname on which the API server is serving
	// +optional
	Host string `json:"host,omitempty"`

	// Port is the port on which the API server is serving
	// +optional
	Port int32 `json:"port,omitempty"`
}

// FreeboxClusterStatus defines the observed state of FreeboxCluster
type FreeboxClusterStatus struct {
	// initialization provides observations of the FreeboxCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization *FreeboxClusterInitializationStatus `json:"initialization,omitempty"`

	// conditions represent the current state of the FreeboxCluster resource.
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
}

// FreeboxClusterInitializationStatus provides observations of the FreeboxCluster initialization process.
// +kubebuilder:validation:MinProperties=1
type FreeboxClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the Cluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FreeboxCluster is the Schema for the freeboxclusters API
type FreeboxCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of FreeboxCluster
	// +required
	Spec FreeboxClusterSpec `json:"spec"`

	// status defines the observed state of FreeboxCluster
	// +optional
	Status FreeboxClusterStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// FreeboxClusterList contains a list of FreeboxCluster
type FreeboxClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FreeboxCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &FreeboxCluster{}, &FreeboxClusterList{})
}
