// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package preview

import (
	"fmt"
	"time"

	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateBackup generates a new Backup resource for this ScheduledBackup.
// The backup name is generated with a timestamp suffix to ensure uniqueness.
func (scheduledBackup *ScheduledBackup) CreateBackup(now time.Time) *Backup {
	// Generate backup name with timestamp
	backupName := fmt.Sprintf("%s-%s", scheduledBackup.Name, now.Format("20060102-150405"))

	return &Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: scheduledBackup.Namespace,
			Labels: map[string]string{
				"scheduledbackup": scheduledBackup.Name,
			},
		},
		Spec: BackupSpec{
			Cluster:       scheduledBackup.Spec.Cluster,
			RetentionDays: scheduledBackup.Spec.RetentionDays,
		},
	}
}

// GetNextScheduleTime calculates the next scheduled time
func (scheduledBackup *ScheduledBackup) GetNextScheduleTime(schedule cron.Schedule, lastBackup *Backup) time.Time {
	// If there is a last backup, calculate the next schedule time based on its creation time
	if lastBackup != nil && lastBackup.CreationTimestamp.Time.After(time.Time{}) {
		return schedule.Next(lastBackup.CreationTimestamp.Time)
	}

	if scheduledBackup.Status.NextScheduledTime != nil {
		return scheduledBackup.Status.NextScheduledTime.Time
	}

	return schedule.Next(time.Now())
}
