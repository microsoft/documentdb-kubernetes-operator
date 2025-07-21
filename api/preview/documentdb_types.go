// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package preview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DocumentDBSpec defines the desired state of DocumentDB.
type DocumentDBSpec struct {
	// NodeCount is the number of nodes in the DocumentDB cluster. Must be 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	NodeCount int `json:"nodeCount"`

	// InstancesPerNode is the number of DocumentDB instances per node. Must be 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	InstancesPerNode int `json:"instancesPerNode"`

	// Resource specifies the storage resources for DocumentDB.
	Resource Resource `json:"resource"`

	// DocumentDBImage is the container image to use for DocumentDB.
	DocumentDBImage string `json:"documentDBImage"`

	// ClusterReplication configures cross-cluster replication for DocumentDB.
	ClusterReplication *ClusterReplication `json:"clusterReplication,omitempty"`

	// SidecarInjectorPluginName is the name of the sidecar injector plugin to use.
	SidecarInjectorPluginName string `json:"sidecarInjectorPluginName,omitempty"`

	// ExposeViaService configures how to expose DocumentDB via a Kubernetes service.
	// This can be a LoadBalancer or ClusterIP service.
	ExposeViaService ExposeViaService `json:"exposeViaService,omitempty"`

	Timeouts Timeouts `json:"timeouts,omitempty"`
}

type Resource struct {
	// PvcSize is the size of the persistent volume claim for DocumentDB storage (e.g., "10Gi").
	PvcSize string `json:"pvcSize"`
}

type ClusterReplication struct {
	// Primary is the name of the primary cluster for replication.
	Primary string `json:"primary"`
	// ClusterList is the list of clusters participating in replication.
	ClusterList []string `json:"clusterList"`
}

type ExposeViaService struct {
	// ServiceType determines the type of service to expose for DocumentDB.
	// +kubebuilder:validation:Enum=LoadBalancer;ClusterIP
	ServiceType string `json:"serviceType"`
}

type Timeouts struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1800
	StopDelay int32 `json:"stopDelay,omitempty"`
}

// DocumentDBStatus defines the observed state of DocumentDB.
type DocumentDBStatus struct {
	// Status reflects the status field from the underlying CNPG Cluster.
	Status           string `json:"status,omitempty"`
	ConnectionString string `json:"connectionString,omitempty"`
}

// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status",description="CNPG Cluster Status"
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
