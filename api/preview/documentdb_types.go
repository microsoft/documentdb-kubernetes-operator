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

	// DocumentDBVersion specifies the version for all DocumentDB components (engine, gateway).
	// When set, this overrides the default versions for documentDBImage and gatewayImage.
	// Individual image fields take precedence over this version.
	DocumentDBVersion string `json:"documentDBVersion,omitempty"`

	// DocumentDBImage is the container image to use for DocumentDB.
	// Changing this is not recommended for most users.
	// If not specified, defaults based on documentDBVersion or operator defaults.
	DocumentDBImage string `json:"documentDBImage,omitempty"`

	// GatewayImage is the container image to use for the DocumentDB Gateway sidecar.
	// Changing this is not recommended for most users.
	// If not specified, defaults to a version that matches the DocumentDB operator version.
	GatewayImage string `json:"gatewayImage,omitempty"`

	// DocumentDbCredentialSecret is the name of the Kubernetes Secret containing credentials
	// for the DocumentDB gateway (expects keys `username` and `password`). If omitted,
	// a default secret name `documentdb-credentials` is used.
	DocumentDbCredentialSecret string `json:"documentDbCredentialSecret,omitempty"`

	// ClusterReplication configures cross-cluster replication for DocumentDB.
	ClusterReplication *ClusterReplication `json:"clusterReplication,omitempty"`

	// SidecarInjectorPluginName is the name of the sidecar injector plugin to use.
	SidecarInjectorPluginName string `json:"sidecarInjectorPluginName,omitempty"`

	// ExposeViaService configures how to expose DocumentDB via a Kubernetes service.
	// This can be a LoadBalancer or ClusterIP service.
	ExposeViaService ExposeViaService `json:"exposeViaService,omitempty"`

	Timeouts Timeouts `json:"timeouts,omitempty"`

	// TLS configures (future) gateway TLS certificate management. Phase 1: create cert-manager resources and status tracking only.
	TLS *GatewayTLS `json:"tls,omitempty"`
}

type Resource struct {
	// PvcSize is the size of the persistent volume claim for DocumentDB storage (e.g., "10Gi").
	PvcSize string `json:"pvcSize"`
}

type ClusterReplication struct {
	// EnableFleetForCrossCloud determines whether to use KubeFleet mechanics for the replication
	EnableFleetForCrossCloud bool `json:"enableFleetForCrossCloud,omitempty"`
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

// GatewayTLS defines TLS configuration for the gateway sidecar (Phase 1: certificate provisioning only)
type GatewayTLS struct {
	// Mode selects the TLS management strategy.
	// +kubebuilder:validation:Enum=Disabled;SelfSigned;CertManager;Provided
	Mode string `json:"mode,omitempty"`

	// CertManager config when Mode=CertManager.
	CertManager *CertManagerTLS `json:"certManager,omitempty"`

	// Provided secret reference when Mode=Provided.
	Provided *ProvidedTLS `json:"provided,omitempty"`
}

// CertManagerTLS holds parameters for cert-manager driven certificates.
type CertManagerTLS struct {
	IssuerRef IssuerRef `json:"issuerRef"`
	// DNSNames for the certificate SANs. If empty, operator will add Service DNS names.
	DNSNames []string `json:"dnsNames,omitempty"`
	// SecretName optional explicit name for the target secret. If empty a default is chosen.
	SecretName string `json:"secretName,omitempty"`
}

// ProvidedTLS references an existing secret that contains tls.crt/tls.key (and optional ca.crt).
type ProvidedTLS struct {
	SecretName string `json:"secretName"`
}

// IssuerRef references a cert-manager Issuer or ClusterIssuer.
type IssuerRef struct {
	Name string `json:"name"`
	// Kind of issuer (Issuer or ClusterIssuer). Defaults to Issuer.
	Kind string `json:"kind,omitempty"`
	// Group defaults to cert-manager.io
	Group string `json:"group,omitempty"`
}

// DocumentDBStatus defines the observed state of DocumentDB.
type DocumentDBStatus struct {
	// Status reflects the status field from the underlying CNPG Cluster.
	Status           string `json:"status,omitempty"`
	ConnectionString string `json:"connectionString,omitempty"`

	// TLS reports gateway TLS provisioning status (Phase 1).
	TLS *TLSStatus `json:"tls,omitempty"`
}

// TLSStatus captures readiness and secret information.
type TLSStatus struct {
	Ready      bool   `json:"ready,omitempty"`
	SecretName string `json:"secretName,omitempty"`
	Message    string `json:"message,omitempty"`
}

// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status",description="CNPG Cluster Status"
// +kubebuilder:printcolumn:name="Connection String",type=string,JSONPath=".status.connectionString",description="DocumentDB Connection String"
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
