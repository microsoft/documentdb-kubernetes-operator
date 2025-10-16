// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

var _ = Describe("ScheduledBackup Controller", func() {
	const (
		scheduledBackupName      = "test-scheduled-backup"
		scheduledBackupNamespace = "default"
		clusterName              = "test-cluster"
	)

	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(dbpreview.AddToScheme(scheme)).To(Succeed())
	})

	It("returns error for invalid cron schedule", func() {
		invalidSchedule := "invalid cron expression"
		scheduledBackup := &dbpreview.ScheduledBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scheduledBackupName,
				Namespace: scheduledBackupNamespace,
			},
			Spec: dbpreview.ScheduledBackupSpec{
				Schedule: invalidSchedule,
				Cluster: cnpgv1.LocalObjectReference{
					Name: clusterName,
				},
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(scheduledBackup).
			Build()

		reconciler := &ScheduledBackupReconciler{
			Client: fakeClient,
			Scheme: scheme,
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      scheduledBackupName,
				Namespace: scheduledBackupNamespace,
			},
		})

		// expect err: invalid cron expression
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid cron expression"))
		Expect(result.Requeue).To(BeFalse())
	})

	Describe("isBackupRunning", func() {
		It("returns true when any backup is in a non-terminal phase", func() {
			r := &ScheduledBackupReconciler{}

			backupRunning := dbpreview.Backup{
				Status: dbpreview.BackupStatus{
					Phase: cnpgv1.BackupPhaseRunning,
				},
			}
			backupList := &dbpreview.BackupList{Items: []dbpreview.Backup{backupRunning}}
			Expect(r.isBackupRunning(backupList)).To(BeTrue())
		})

		It("returns false for an empty backup list", func() {
			r := &ScheduledBackupReconciler{}
			Expect(r.isBackupRunning(&dbpreview.BackupList{Items: []dbpreview.Backup{}})).To(BeFalse())
		})

		It("returns false when all backups are terminal or empty phase", func() {
			r := &ScheduledBackupReconciler{}

			backupCompleted := dbpreview.Backup{
				Status: dbpreview.BackupStatus{
					Phase: cnpgv1.BackupPhaseCompleted,
				},
			}
			backupFailed := dbpreview.Backup{
				Status: dbpreview.BackupStatus{
					Phase: cnpgv1.BackupPhaseFailed,
				},
			}
			backupEmpty := dbpreview.Backup{
				Status: dbpreview.BackupStatus{
					Phase: cnpgv1.BackupPhase(""),
				},
			}
			backupList := &dbpreview.BackupList{Items: []dbpreview.Backup{backupCompleted, backupFailed, backupEmpty}}
			Expect(r.isBackupRunning(backupList)).To(BeFalse())
		})
	})

	Describe("getNextScheduleTime", func() {
		It("returns next time based on creation time when no backups exist", func() {
			schedule, err := cron.ParseStandard("0 0 * * *")
			Expect(err).NotTo(HaveOccurred())

			r := &ScheduledBackupReconciler{}
			scheduledBackupCreationTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

			nextFromCreation := r.getNextScheduleTime(schedule, scheduledBackupCreationTime, &dbpreview.BackupList{Items: []dbpreview.Backup{}})
			Expect(nextFromCreation).To(Equal(schedule.Next(scheduledBackupCreationTime)))
		})

		It("returns next time based on the last backup creation timestamp when backups exist", func() {
			schedule, err := cron.ParseStandard("0 0 * * *")
			Expect(err).NotTo(HaveOccurred())

			r := &ScheduledBackupReconciler{}

			t1 := time.Date(2025, 10, 11, 1, 0, 0, 0, time.UTC)
			t2 := time.Date(2025, 10, 12, 2, 0, 0, 0, time.UTC)
			backupList := &dbpreview.BackupList{
				Items: []dbpreview.Backup{
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: t1},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: t2},
						},
					},
				},
			}
			nextFromLastBackup := r.getNextScheduleTime(schedule, time.Time{}, backupList)
			Expect(nextFromLastBackup).To(Equal(schedule.Next(t2)))
		})
	})

	Describe("createBackup", func() {
		It("creates a Backup with expected fields", func() {
			// fake client with no existing backups
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			reconciler := &ScheduledBackupReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			scheduledBackup := &dbpreview.ScheduledBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      scheduledBackupName,
					Namespace: scheduledBackupNamespace,
				},
				Spec: dbpreview.ScheduledBackupSpec{
					Schedule: "0 0 * * *",
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			err := reconciler.createBackup(ctx, scheduledBackup)
			Expect(err).NotTo(HaveOccurred())

			backupList := &dbpreview.BackupList{}
			err = fakeClient.List(ctx, backupList)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(backupList.Items)).To(Equal(1))

			backup := backupList.Items[0]

			// name prefix and parseable timestamp suffix
			prefix := scheduledBackupName + "-"
			Expect(strings.HasPrefix(backup.Name, prefix)).To(BeTrue())

			suffix := strings.TrimPrefix(backup.Name, prefix)
			const layout = "20060102-150405"
			_, parseErr := time.Parse(layout, suffix)
			Expect(parseErr).NotTo(HaveOccurred())

			// namespace
			Expect(backup.Namespace).To(Equal(scheduledBackupNamespace))

			// label
			Expect(backup.Labels).To(HaveKeyWithValue("scheduledbackup", scheduledBackupName))

			// cluster reference
			Expect(backup.Spec.Cluster.Name).To(Equal(clusterName))
		})
	})
})
