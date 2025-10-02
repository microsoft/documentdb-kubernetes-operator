// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=db.microsoft.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=db.microsoft.com,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=db.microsoft.com,resources=backups/finalizers,verbs=update

// Reconcile handles the reconciliation loop for Backup resources.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	backup := &dbpreview.Backup{}
	err := r.Get(ctx, req.NamespacedName, backup)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get Backup")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cnpgBackup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: cnpgv1.BackupSpec{
			Method: cnpgv1.VolumeSnapshotKind,
			Cluster: cnpgv1.LocalObjectReference{
				Name: backup.Spec.Cluster.Name,
			},
		},
	}

	// Create the CNPG Backup
	if err := r.Create(ctx, cnpgBackup); err != nil {
		log.FromContext(ctx).Error(err, "Failed to create CNPG Backup")
		return ctrl.Result{}, err
	}

	// Update the status of the Backup resource with cpng backup status

	backup.Status.Phase = string(cnpgBackup.Status.Phase)
	backup.Status.StartedAt = cnpgBackup.Status.StartedAt
	backup.Status.StoppedAt = cnpgBackup.Status.StoppedAt

	if err := r.Status().Update(ctx, backup); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update Backup status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbpreview.Backup{}).
		Owns(&cnpgv1.Backup{}).
		Complete(r)
}
