// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var cfg *rest.Config
var testEnv *envtest.Environment
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

type testingEnvironment struct {
	backupReconciler     *BackupReconciler
	documentDBReconciler *DocumentDBReconciler
}

func buildTestEnvironment() *testingEnvironment {
	var err error
	Expect(err).ToNot(HaveOccurred())

	k8sClient := fake.NewClientBuilder().
		Build()
	Expect(err).ToNot(HaveOccurred())

	documentDBReconciler := &DocumentDBReconciler{
		Client: k8sClient,
	}

	backupReconciler := &BackupReconciler{
		Client: k8sClient,
	}

	return &testingEnvironment{
		documentDBReconciler: documentDBReconciler,
		backupReconciler:     backupReconciler,
	}
}
