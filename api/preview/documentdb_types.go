// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License. See LICENSE file in the project root for full license information.

package preview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DocumentDBSpec defines the desired state of DocumentDB.
type DocumentDBSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	NodeCount int `json:"nodeCount"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	InstancesPerNode    int                  `json:"instancesPerNode"`
	Resource            Resource             `json:"resource"`
	DocumentDBImage     string               `json:"documentDBImage"`
	PhysicalReplication *PhysicalReplication `json:"physicalReplication,omitempty"`
	CNPGSidecarPlugin   string               `json:"cnpgSidecarPlugin,omitempty"`
}

type Resource struct {
	PvcSize string `json:"pvcSize"`
}

type PhysicalReplication struct {
	Primary     string   `json:"primary"`
	ClusterList []string `json:"clusterList"`
}

// DocumentDBStatus defines the observed state of DocumentDB.
type DocumentDBStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DocumentDB is the Schema for the documentdbs API.
type DocumentDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DocumentDBSpec   `json:"spec,omitempty"`
	Status DocumentDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DocumentDBList contains a list of DocumentDB.
type DocumentDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DocumentDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DocumentDB{}, &DocumentDBList{})
}
