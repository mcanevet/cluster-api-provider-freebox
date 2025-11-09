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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FreeboxClusterSpec defines the desired state of FreeboxCluster
type FreeboxClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// This is required and must be set by the user to the actual control plane endpoint.
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// FreeboxClusterStatus defines the observed state of FreeboxCluster.
type FreeboxClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Ready is true when the provider resource is ready.
	// NOTE: This field is part of the Cluster API contract and is required for the Cluster to be considered ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// initialization provides observations of the FreeboxCluster initialization process.
	// NOTE: This field is part of the Cluster API contract and is used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization FreeboxClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

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
// +kubebuilder:resource:path=freeboxclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this FreeboxCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.initialization.provisioned",description="FreeboxCluster ready status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of FreeboxCluster"

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
