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
	// The cluster must exist in the same namespace as the Backup resource.
	// +kubebuilder:validation:Required
	Cluster cnpgv1.LocalObjectReference `json:"cluster"`

	// RetentionDays specifies how many days the backup should be retained.
	// If not specified, the default retention period from the cluster's backup policy is used.
	// +optional
	RetentionDays *int `json:"retentionDays,omitempty"`
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

	// ExpiredAt is the time when the backup is considered expired and can be deleted.
	// +optional
	ExpiredAt *metav1.Time `json:"expiredAt,omitempty"`

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
// +kubebuilder:printcolumn:name="Expired",type=date,JSONPath=".status.expiredAt",description="Backup expiration time"
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=".status.error",description="Backup error information"
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
