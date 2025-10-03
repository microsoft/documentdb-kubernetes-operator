// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

var (
	k8sClient client.Client
	ctx       context.Context
)

var _ = Describe("Backup controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var namespace *corev1.Namespace
	var cluster *dbpreview.DocumentDB

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-test-" + randomString(6),
			},
		}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

		cluster = &dbpreview.DocumentDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: namespace.Name,
			},
			Spec: dbpreview.DocumentDBSpec{
				NodeCount:        1,
				InstancesPerNode: 1,
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	Context("when creating a Backup", func() {
		var backup *dbpreview.Backup

		BeforeEach(func() {
			backup = &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup",
					Namespace: namespace.Name,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: dbpreview.LocalObjectReference{
						Name: cluster.Name,
					},
				},
			}
		})

		It("should create a CNPG Backup resource", func() {
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			Eventually(func() error {
				cnpgBackup := &cnpgv1.Backup{}
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				}, cnpgBackup)
			}, timeout, interval).Should(Succeed())

			cnpgBackup := &cnpgv1.Backup{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      backup.Name,
				Namespace: backup.Namespace,
			}, cnpgBackup)).To(Succeed())

			Expect(cnpgBackup.Spec.Cluster.Name).To(Equal(cluster.Name))
			Expect(cnpgBackup.Spec.Method).To(Equal(cnpgv1.VolumeSnapshotKind))
		})

		It("should update status when CNPG backup status changes", func() {
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Wait for CNPG backup to be created
			cnpgBackup := &cnpgv1.Backup{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				}, cnpgBackup)
			}, timeout, interval).Should(Succeed())

			// Simulate CNPG backup status update
			now := metav1.Now()
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseRunning
			cnpgBackup.Status.StartedAt = &now
			Expect(k8sClient.Status().Update(ctx, cnpgBackup)).To(Succeed())

			// Trigger reconciliation
			reconciler := &BackupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			// Verify backup status is updated
			Eventually(func() string {
				updatedBackup := &dbpreview.Backup{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				}, updatedBackup)
				if err != nil {
					return ""
				}
				return updatedBackup.Status.Phase
			}, timeout, interval).Should(Equal(string(cnpgv1.BackupPhaseRunning)))
		})

		It("should handle completed backup status", func() {
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Wait for CNPG backup to be created
			cnpgBackup := &cnpgv1.Backup{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				}, cnpgBackup)
			}, timeout, interval).Should(Succeed())

			// Simulate completed backup
			startTime := metav1.Now()
			stopTime := metav1.NewTime(startTime.Add(5 * time.Minute))
			cnpgBackup.Status.Phase = cnpgv1.BackupPhaseCompleted
			cnpgBackup.Status.StartedAt = &startTime
			cnpgBackup.Status.StoppedAt = &stopTime
			Expect(k8sClient.Status().Update(ctx, cnpgBackup)).To(Succeed())

			// Trigger reconciliation
			reconciler := &BackupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				},
			})
			Expect(err).ToNot(HaveOccurred())

			// Verify backup status
			updatedBackup := &dbpreview.Backup{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      backup.Name,
				Namespace: backup.Namespace,
			}, updatedBackup)).To(Succeed())

			Expect(updatedBackup.Status.Phase).To(Equal(string(cnpgv1.BackupPhaseCompleted)))
			Expect(updatedBackup.Status.StartedAt).ToNot(BeNil())
			Expect(updatedBackup.Status.StoppedAt).ToNot(BeNil())
		})
	})

	Context("when backup is deleted", func() {
		It("should handle backup not found gracefully", func() {
			reconciler := &BackupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-backup",
					Namespace: namespace.Name,
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when CNPG backup already exists", func() {
		It("should return error on conflict", func() {
			backup := &dbpreview.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-backup",
					Namespace: namespace.Name,
				},
				Spec: dbpreview.BackupSpec{
					Cluster: dbpreview.LocalObjectReference{
						Name: cluster.Name,
					},
				},
			}

			// Create CNPG backup first
			cnpgBackup := &cnpgv1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				},
				Spec: cnpgv1.BackupSpec{
					Method: cnpgv1.VolumeSnapshotKind,
					Cluster: cnpgv1.LocalObjectReference{
						Name: cluster.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, cnpgBackup)).To(Succeed())

			// Create backup resource
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Reconcile should fail due to existing CNPG backup
			reconciler := &BackupReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      backup.Name,
					Namespace: backup.Namespace,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(apierrs.IsAlreadyExists(err)).To(BeTrue())
		})
	})
})

// Helper function to generate random strings for unique namespaces
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
