// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

const (
	requeueAfterShort = 10 * time.Second
	requeueAfterLong  = 30 * time.Second
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for Backup resources.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Backup resource
	backup := &dbpreview.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Backup")
		return ctrl.Result{}, err
	}

	// Get or create the CNPG Backup
	cnpgBackup := &cnpgv1.Backup{}
	cnpgBackupKey := client.ObjectKey{
		Name:      backup.Name,
		Namespace: backup.Namespace,
	}

	err := r.Get(ctx, cnpgBackupKey, cnpgBackup)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create CNPG Backup
			return r.createCNPGBackup(ctx, backup)
		}
		logger.Error(err, "Failed to get CNPG Backup")
		return ctrl.Result{}, err
	}

	// Update status based on CNPG Backup status
	return r.updateBackupStatus(ctx, backup, cnpgBackup)
}

// createCNPGBackup creates a new CNPG Backup resource
func (r *BackupReconciler) createCNPGBackup(ctx context.Context, backup *dbpreview.Backup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cnpgBackup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: cnpgv1.BackupSpec{
			Method: cnpgv1.BackupMethodVolumeSnapshot,
			Cluster: cnpgv1.LocalObjectReference{
				Name: backup.Spec.Cluster.Name,
			},
		},
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(backup, cnpgBackup, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference on CNPG Backup")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, cnpgBackup); err != nil {
		logger.Error(err, "Failed to create CNPG Backup")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully created CNPG Backup", "name", cnpgBackup.Name)

	// Requeue to check status
	return ctrl.Result{RequeueAfter: requeueAfterShort}, nil
}

// updateBackupStatus updates the Backup status based on CNPG Backup status
func (r *BackupReconciler) updateBackupStatus(ctx context.Context, backup *dbpreview.Backup, cnpgBackup *cnpgv1.Backup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if status needs update
	needsUpdate := false
	newPhase := cnpgBackup.Status.Phase

	if backup.Status.Phase != newPhase {
		backup.Status.Phase = newPhase
		backup.Status.Error = cnpgBackup.Status.Error
		needsUpdate = true
	}

	if !areTimesEqual(backup.Status.StartedAt, cnpgBackup.Status.StartedAt) {
		backup.Status.StartedAt = cnpgBackup.Status.StartedAt
		needsUpdate = true
	}

	if !areTimesEqual(backup.Status.StoppedAt, cnpgBackup.Status.StoppedAt) {
		backup.Status.StoppedAt = cnpgBackup.Status.StoppedAt
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.Status().Update(ctx, backup); err != nil {
			logger.Error(err, "Failed to update Backup status")
			return ctrl.Result{}, err
		}
	}

	// Determine requeue behavior based on phase
	if cnpgBackup.Status.Phase == cnpgv1.BackupPhaseCompleted {
		logger.Info("Backup completed", "phase", newPhase, "name", backup.Name)
		// Stop reconciling - backup is complete
		return ctrl.Result{}, nil
	}

	if cnpgBackup.Status.Phase == cnpgv1.BackupPhaseFailed {
		logger.Error(nil, "Backup failed", "phase", newPhase, "name", backup.Name)
		// Stop reconciling - backup has failed
		return ctrl.Result{}, nil
	}

	// Backup is still in progress, requeue to check status again
	return ctrl.Result{RequeueAfter: requeueAfterLong}, nil
}

// Helper functions

// areTimesEqual compares two metav1.Time pointers for equality
func areTimesEqual(t1, t2 *metav1.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(t2)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbpreview.Backup{}).
		Owns(&cnpgv1.Backup{}).
		Complete(r)
}
