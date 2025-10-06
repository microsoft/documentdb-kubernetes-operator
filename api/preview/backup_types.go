// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package preview

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup.
type BackupSpec struct {
	// Cluster specifies the DocumentDB cluster to backup.
	Cluster cnpgv1.LocalObjectReference `json:"cluster"`
}

// BackupStatus defines the observed state of Backup.
type BackupStatus struct {
	// Phase represents the current phase of the backup operation.
	Phase cnpgv1.BackupPhase `json:"phase,omitempty"`

	// StartedAt is the time when the backup operation started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// StoppedAt is the time when the backup operation completed or failed.
	// +optional
	StoppedAt *metav1.Time `json:"stoppedAt,omitempty"`

	// Error contains error information if the backup failed.
	// +optional
	Error string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.cluster.name",description="Target DocumentDB cluster"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase",description="Backup phase"
// +kubebuilder:printcolumn:name="Started",type=date,JSONPath=".status.startedAt",description="Backup start time"
// +kubebuilder:printcolumn:name="Stopped",type=date,JSONPath=".status.stoppedAt",description="Backup completion time"
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=".status.error",description="Backup error information"

// Backup is the Schema for the backups API.
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
