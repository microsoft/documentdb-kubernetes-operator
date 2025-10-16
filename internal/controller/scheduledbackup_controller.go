// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"fmt"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

// ScheduledBackupReconciler reconciles a ScheduledBackup object
type ScheduledBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for ScheduledBackup resources.
func (r *ScheduledBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ScheduledBackup resource
	scheduledBackup := &dbpreview.ScheduledBackup{}
	if err := r.Get(ctx, req.NamespacedName, scheduledBackup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ScheduledBackup")
		return ctrl.Result{}, err
	}

	// Parse cron schedule
	schedule, err := cron.ParseStandard(scheduledBackup.Spec.Schedule)
	if err != nil {
		logger.Error(err, "Invalid cron schedule", "schedule", scheduledBackup.Spec.Schedule)
		return ctrl.Result{}, err
	}

	backupList := &dbpreview.BackupList{}
	if err := r.List(ctx, backupList, client.InNamespace(scheduledBackup.Namespace), client.MatchingFields{"spec.cluster": scheduledBackup.Spec.Cluster.Name}); err != nil {
		logger.Error(err, "Failed to list backups")
		// Requeue and try again shortly on list errors
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if r.isBackupRunning(backupList) {
		// If a backup is currently running, requeue after a short delay
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Calculate next schedule time
	nextScheduleTime := r.getNextScheduleTime(schedule, scheduledBackup.CreationTimestamp.Time, backupList)

	// If it's time to create a backup
	now := time.Now()
	if !now.Before(nextScheduleTime) {
		if err := r.createBackup(ctx, scheduledBackup); err != nil {
			logger.Error(err, "Failed to create backup")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// Calculate next run time
		nextScheduleTime = schedule.Next(now)
	}

	// Requeue at next schedule time
	requeueAfter := time.Until(nextScheduleTime)
	if requeueAfter < 0 {
		requeueAfter = time.Minute
	}

	logger.Info("Next backup scheduled", "requeueAfter", requeueAfter, "nextTime", nextScheduleTime)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *ScheduledBackupReconciler) isBackupRunning(backupList *dbpreview.BackupList) bool {
	for _, backup := range backupList.Items {
		if backup.Status.Phase != "" && backup.Status.Phase != cnpgv1.BackupPhaseCompleted && backup.Status.Phase != cnpgv1.BackupPhaseFailed {
			return true
		}
	}
	return false
}

func (r *ScheduledBackupReconciler) getNextScheduleTime(schedule cron.Schedule, scheduledBackupCreationTime time.Time, backupList *dbpreview.BackupList) time.Time {
	if backupList == nil || len(backupList.Items) == 0 {
		return schedule.Next(scheduledBackupCreationTime)
	}

	lastBackupStartTime := time.Time{}
	for _, backup := range backupList.Items {
		if backup.CreationTimestamp.After(lastBackupStartTime) {
			lastBackupStartTime = backup.CreationTimestamp.Time
		}
	}
	return schedule.Next(lastBackupStartTime)
}

// createBackup creates a new Backup resource for this scheduled backup
func (r *ScheduledBackupReconciler) createBackup(ctx context.Context, scheduledBackup *dbpreview.ScheduledBackup) error {
	logger := log.FromContext(ctx)

	// Generate backup name with timestamp
	backupName := fmt.Sprintf("%s-%s", scheduledBackup.Name, time.Now().Format("20060102-150405"))

	backup := &dbpreview.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: scheduledBackup.Namespace,
			Labels: map[string]string{
				"scheduledbackup": scheduledBackup.Name,
			},
		},
		Spec: dbpreview.BackupSpec{
			Cluster: scheduledBackup.Spec.Cluster,
		},
	}

	if err := r.Create(ctx, backup); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info("Backup already exists", "name", backupName)
			return nil
		}
		logger.Error(err, "Failed to create Backup")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduledBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbpreview.ScheduledBackup{}).
		Complete(r)
}
