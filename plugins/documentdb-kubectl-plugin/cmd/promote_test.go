package cmd

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestWaitForPromotion(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: documentDBGVRGroup, Version: documentDBGVRVersion, Kind: "DocumentDB"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("DocumentDBList"), &unstructured.UnstructuredList{})

	namespace := defaultDocumentDBNamespace
	docName := "sample"
	targetCluster := "cluster-b"

	hubDoc := newDocument(docName, namespace, "cluster-a", "Creating")
	targetDoc := newDocument(docName, namespace, "cluster-a", "Creating")

	hubClient := dynamicfake.NewSimpleDynamicClient(scheme, hubDoc.DeepCopy())
	targetClient := dynamicfake.NewSimpleDynamicClient(scheme, targetDoc.DeepCopy())

	opts := &promoteOptions{
		documentDBName: docName,
		namespace:      namespace,
		targetCluster:  targetCluster,
		waitTimeout:    500 * time.Millisecond,
		pollInterval:   20 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	gvr := schema.GroupVersionResource{Group: documentDBGVRGroup, Version: documentDBGVRVersion, Resource: documentDBGVRResource}

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(60 * time.Millisecond)
		if err := setDocumentState(ctx, hubClient, gvr, namespace, docName, targetCluster, "Ready"); err != nil {
			errCh <- err
			return
		}
		if err := setDocumentState(ctx, targetClient, gvr, namespace, docName, targetCluster, "Ready"); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	if err := opts.waitForPromotion(ctx, hubClient, targetClient); err != nil {
		t.Fatalf("waitForPromotion returned error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("failed to update documents: %v", err)
	}
}

func TestPatchDocumentDB(t *testing.T) {
	t.Parallel()

	scheme := newDocumentScheme()
	gvr := documentDBGVR()

	namespace := defaultDocumentDBNamespace
	docName := "sample"

	doc := newDocument(docName, namespace, "cluster-a", "Ready")

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, documentListKinds(), doc.DeepCopy())

	opts := &promoteOptions{
		documentDBName: docName,
		namespace:      namespace,
		targetCluster:  "cluster-b",
	}

	if err := opts.patchDocumentDB(context.Background(), client); err != nil {
		t.Fatalf("patchDocumentDB returned error: %v", err)
	}

	patched, err := client.Resource(gvr).Namespace(namespace).Get(context.Background(), docName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to fetch patched document: %v", err)
	}

	primary, _, err := unstructured.NestedString(patched.Object, "spec", "clusterReplication", "primary")
	if err != nil {
		t.Fatalf("failed to read patched primary: %v", err)
	}
	if primary != "cluster-b" {
		t.Fatalf("expected primary cluster-b, got %q", primary)
	}
}

func setDocumentState(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name, primary, phase string) error {
	for {
		obj, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedField(obj.Object, primary, "spec", "clusterReplication", "primary"); err != nil {
			return err
		}
		if err := unstructured.SetNestedField(obj.Object, phase, "status", "status"); err != nil {
			return err
		}
		_, err = client.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
}
