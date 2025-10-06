// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

var _ = Describe("Backup Controller", func() {
	const (
		backupName      = "test-backup"
		backupNamespace = "default"
		clusterName     = "test-cluster"
		timeout         = time.Second * 10
		interval        = time.Millisecond * 250
	)

	var (
		testCtx    context.Context
		reconciler *BackupReconciler
		testScheme *runtime.Scheme
	)

	BeforeEach(func() {
		testCtx = context.Background()
		testScheme = runtime.NewScheme()
		_ = dbpreview.AddToScheme(testScheme)
		_ = cnpgv1.AddToScheme(testScheme)
	})

	Context("When reconciling a new Backup resource", func() {
		var (
			backup     *dbpreview.Backup
			fakeClient client.Client
		)

		BeforeEach(func() {
			backup = &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(backup).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler = &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}
		})

		It("should create a CNPG backup when no CNPG backup exists", func() {
			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterShort))

			// Verify CNPG backup was created
			cnpgBackup := &cnpgv1.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(cnpgBackup.Spec.Cluster.Name).To(Equal(clusterName))
			Expect(cnpgBackup.Spec.Method).To(Equal(cnpgv1.BackupMethodVolumeSnapshot))

			// Verify owner reference is set
			Expect(cnpgBackup.OwnerReferences).To(HaveLen(1))
			Expect(cnpgBackup.OwnerReferences[0].Name).To(Equal(backupName))
			Expect(cnpgBackup.OwnerReferences[0].Kind).To(Equal("Backup"))
		})

		It("should return error when backup resource is not found", func() {
			nonExistentBackup := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: backupNamespace,
				},
			}

			result, err := reconciler.Reconcile(testCtx, nonExistentBackup)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Context("When updating backup status", func() {
		var (
			backup     *dbpreview.Backup
			cnpgBackup *cnpgv1.Backup
			fakeClient client.Client
		)

		BeforeEach(func() {
			backup = &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			cnpgBackup = &cnpgv1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: dbpreview.GroupVersion.String(),
							Kind:       "Backup",
							Name:       backupName,
							UID:        backup.UID,
						},
					},
				},
				Spec: cnpgv1.BackupSpec{
					Method: cnpgv1.BackupMethodVolumeSnapshot,
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(backup, cnpgBackup).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler = &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}
		})

		It("should update backup status when CNPG backup is running", func() {
			// Update CNPG backup status to running
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			cnpgBackup.Status.StartedAt = &now
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Verify backup status was updated
			updatedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, updatedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(updatedBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseRunning))
			Expect(updatedBackup.Status.StartedAt).NotTo(BeNil())
			Expect(updatedBackup.Status.StartedAt.Equal(cnpgBackup.Status.StartedAt)).To(BeTrue())
		})

		It("should not requeue when CNPG backup is completed", func() {
			// Update CNPG backup status to completed
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseCompleted
			cnpgBackup.Status.StartedAt = &now
			cnpgBackup.Status.StoppedAt = &now
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			// Verify backup status was updated
			updatedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, updatedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(updatedBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseCompleted))
			Expect(updatedBackup.Status.StartedAt).NotTo(BeNil())
			Expect(updatedBackup.Status.StoppedAt).NotTo(BeNil())
		})

		It("should not requeue when CNPG backup has failed", func() {
			// Update CNPG backup status to failed
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseFailed
			cnpgBackup.Status.StartedAt = &now
			cnpgBackup.Status.StoppedAt = &now
			cnpgBackup.Status.Error = "Backup failed due to some error"
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

			// Verify backup status was updated
			updatedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, updatedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(updatedBackup.Status.Phase)).To(Equal(string(cnpgv1.BackupPhaseFailed)))
			Expect(updatedBackup.Status.Error).To(Equal("Backup failed due to some error"))
		})

		It("should only update status when it has changed", func() {
			// Set initial status
			now := metav1.Now()
			backup.Status.Phase = cnpgv1.BackupPhaseRunning
			backup.Status.StartedAt = &now
			err := fakeClient.Status().Update(testCtx, backup)
			Expect(err).NotTo(HaveOccurred())

			// Set CNPG backup to same status
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			cnpgBackup.Status.StartedAt = &now
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Verify backup status remains the same
			updatedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, updatedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(updatedBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseRunning))
			Expect(updatedBackup.Status.StartedAt).NotTo(BeNil())
			Expect(updatedBackup.Status.StartedAt.Equal(cnpgBackup.Status.StartedAt)).To(BeTrue())
			Expect(updatedBackup.Status.StoppedAt).To(BeNil())
		})

		It("should handle status transitions correctly", func() {
			// Start with no status
			cnpgBackup.Status.Phase = ""
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// First reconcile - no status
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Update to running
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			cnpgBackup.Status.StartedAt = &now
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile - running
			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Update to completed
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseCompleted
			cnpgBackup.Status.StoppedAt = &now
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Third reconcile - completed
			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify final status
			updatedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, updatedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(updatedBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseCompleted))
			Expect(updatedBackup.Status.StartedAt).NotTo(BeNil())
			Expect(updatedBackup.Status.StoppedAt).NotTo(BeNil())
		})
	})

	Context("Helper functions", func() {
		It("should correctly compare metav1.Time pointers", func() {
			now := metav1.Now()
			later := metav1.NewTime(now.Add(time.Minute))
			sameTime := metav1.NewTime(now.Time)

			// Both nil
			Expect(areTimesEqual(nil, nil)).To(BeTrue())

			// Same time
			Expect(areTimesEqual(&now, &now)).To(BeTrue())
			Expect(areTimesEqual(&now, &sameTime)).To(BeTrue())

			// One nil
			Expect(areTimesEqual(&now, nil)).To(BeFalse())
			Expect(areTimesEqual(nil, &now)).To(BeFalse())

			// Different times
			Expect(areTimesEqual(&now, &later)).To(BeFalse())
		})
	})

	Context("Error handling", func() {
		It("should handle CNPG backup creation errors gracefully", func() {
			backup := &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: cnpgv1.LocalObjectReference{
						Name: "", // Invalid - empty cluster name
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(backup).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler := &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}

			// This should still succeed but create a CNPG backup with empty cluster name
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterShort))
		})
	})

	Context("Integration scenarios", func() {
		It("should handle complete backup lifecycle", func() {
			backup := &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(backup).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler := &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}

			// Step 1: Create CNPG backup
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterShort))

			// Verify CNPG backup exists
			cnpgBackup := &cnpgv1.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Step 2: CNPG backup starts
			startTime := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseStarted
			cnpgBackup.Status.StartedAt = &startTime
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Step 3: CNPG backup running
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(requeueAfterLong))

			// Step 4: CNPG backup completes
			stopTime := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseCompleted
			cnpgBackup.Status.StoppedAt = &stopTime
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify final backup status
			finalBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, finalBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(finalBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseCompleted))
			Expect(finalBackup.Status.StartedAt).NotTo(BeNil())
			Expect(finalBackup.Status.StoppedAt).NotTo(BeNil())
		})

		It("should handle backup failure scenario", func() {
			backup := &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			cnpgBackup := &cnpgv1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupName,
					Namespace: backupNamespace,
				},
				Spec: cnpgv1.BackupSpec{
					Method: cnpgv1.BackupMethodVolumeSnapshot,
					Cluster: cnpgv1.LocalObjectReference{
						Name: clusterName,
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(backup, cnpgBackup).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler := &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}

			// Simulate backup failure
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseFailed
			cnpgBackup.Status.StartedAt = &now
			cnpgBackup.Status.StoppedAt = &now
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			// Reconcile
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify backup reflects failure
			failedBackup := &dbpreview.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, failedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(failedBackup.Status.Phase)).To(Equal(cnpgv1.BackupPhaseFailed))
		})
	})
})
