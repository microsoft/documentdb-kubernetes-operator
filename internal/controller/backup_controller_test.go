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

	// Helper function to verify backup status matches CNPG backup status
	verifyBackupStatus := func(client client.Client, cnpgBackup *cnpgv1.Backup) {
		backup := &dbpreview.Backup{}
		err := client.Get(testCtx, types.NamespacedName{
			Name:      backupName,
			Namespace: backupNamespace,
		}, backup)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(backup.Status.Phase)).To(Equal(string(cnpgBackup.Status.Phase)))

		if cnpgBackup.Status.StartedAt != nil {
			Expect(backup.Status.StartedAt).NotTo(BeNil())
			Expect(backup.Status.StartedAt.Equal(cnpgBackup.Status.StartedAt)).To(BeTrue())
		} else {
			Expect(backup.Status.StartedAt).To(BeNil())
		}

		if cnpgBackup.Status.StoppedAt != nil {
			Expect(backup.Status.StoppedAt).NotTo(BeNil())
			Expect(backup.Status.StoppedAt.Equal(cnpgBackup.Status.StoppedAt)).To(BeTrue())
		} else {
			Expect(backup.Status.StoppedAt).To(BeNil())
		}

		Expect(backup.Status.Error).To(Equal(cnpgBackup.Status.Error))
	}

	BeforeEach(func() {
		testCtx = context.Background()
		testScheme = runtime.NewScheme()
		_ = dbpreview.AddToScheme(testScheme)
		_ = cnpgv1.AddToScheme(testScheme)
	})

	Context("Backup lifecycle", func() {
		It("should successfully complete a full backup lifecycle from creation to completion", func() {
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

			// Create CNPG backup
			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      backupName,
					Namespace: backupNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))

			// Verify CNPG backup was created with correct spec and owner reference
			cnpgBackup := &cnpgv1.Backup{}
			err = fakeClient.Get(testCtx, types.NamespacedName{
				Name:      backupName,
				Namespace: backupNamespace,
			}, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(cnpgBackup.Spec.Cluster.Name).To(Equal(clusterName))
			Expect(cnpgBackup.Spec.Method).To(Equal(cnpgv1.BackupMethodVolumeSnapshot))
			Expect(cnpgBackup.OwnerReferences).To(HaveLen(1))
			Expect(cnpgBackup.OwnerReferences[0].Name).To(Equal(backupName))
			Expect(cnpgBackup.OwnerReferences[0].Kind).To(Equal("Backup"))
			Expect(cnpgBackup.OwnerReferences[0].APIVersion).To(Equal(dbpreview.GroupVersion.String()))

			verifyBackupStatus(fakeClient, cnpgBackup)

			// Transition to started phase
			startTime := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseStarted
			cnpgBackup.Status.StartedAt = &startTime
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			verifyBackupStatus(fakeClient, cnpgBackup)

			// Transition to running phase
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			verifyBackupStatus(fakeClient, cnpgBackup)

			// Transition to completed phase
			stopTime := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseCompleted
			cnpgBackup.Status.StoppedAt = &stopTime
			err = fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			verifyBackupStatus(fakeClient, cnpgBackup)
		})

		It("should not requeue when backup resource does not exist", func() {
			fakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithStatusSubresource(&dbpreview.Backup{}, &cnpgv1.Backup{}).
				Build()

			reconciler := &BackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: backupNamespace,
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Context("Status synchronization", func() {
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
				Status: dbpreview.BackupStatus{
					Phase: cnpgv1.BackupPhasePending,
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

		It("should requeue while backup is in progress", func() {
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			now := metav1.Now()
			cnpgBackup.Status.StartedAt = &now
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			verifyBackupStatus(fakeClient, cnpgBackup)

			// Verify reconciliation is idempotent when status hasn't changed
			result, err = reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			verifyBackupStatus(fakeClient, cnpgBackup)
		})

		It("should stop requeuing when backup fails", func() {
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseFailed
			now := metav1.Now()
			cnpgBackup.Status.StartedAt = &now
			cnpgBackup.Status.StoppedAt = &now
			cnpgBackup.Status.Error = "Backup failed due to some error"
			err := fakeClient.Status().Update(testCtx, cnpgBackup)
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: backupName, Namespace: backupNamespace},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			verifyBackupStatus(fakeClient, cnpgBackup)
		})
	})
})
