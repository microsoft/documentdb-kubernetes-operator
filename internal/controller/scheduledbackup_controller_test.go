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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

var _ = Describe("ScheduledBackup Controller", func() {
	const (
		scheduledBackupName      = "test-scheduled-backup"
		scheduledBackupNamespace = "default"
		clusterName              = "test-cluster"
		schedule                 = "0 0 * * *" // Daily at midnight
		timeout                  = time.Second * 10
		interval                 = time.Millisecond * 250
	)

	var (
		testCtx    context.Context
		testScheme *runtime.Scheme
	)

	BeforeEach(func() {
		testCtx = context.Background()
		testScheme = runtime.NewScheme()
		_ = dbpreview.AddToScheme(testScheme)
		_ = cnpgv1.AddToScheme(testScheme)
	})

	Context("ScheduledBackup lifecycle", func() {
		It("throw err for invalid cron schedule", func() {
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
				WithScheme(testScheme).
				WithObjects(scheduledBackup).
				WithStatusSubresource(&dbpreview.ScheduledBackup{}, &cnpgv1.ScheduledBackup{}).
				Build()

			reconciler := &ScheduledBackupReconciler{
				Client: fakeClient,
				Scheme: testScheme,
			}

			result, err := reconciler.Reconcile(testCtx, reconcile.Request{
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
	})
})
