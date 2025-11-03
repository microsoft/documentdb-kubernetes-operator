package cmd

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsDocumentHealthy(t *testing.T) {
	doc := newDocument("test", defaultDocumentDBNamespace, "cluster-a", "Ready")

	healthy, phase := isDocumentHealthy(doc)
	if !healthy {
		t.Fatal("expected document to be healthy")
	}
	if phase != "Ready" {
		t.Fatalf("expected phase to be 'Ready', got %q", phase)
	}

	healthy, _ = isDocumentHealthy(nil)
	if healthy {
		t.Fatal("expected nil document to be unhealthy")
	}
}

func TestIsDocumentReady(t *testing.T) {
	doc := newDocument("test", defaultDocumentDBNamespace, "cluster-a", "Healthy")

	if !isDocumentReady(doc, "cluster-a") {
		t.Fatal("expected document to be ready for cluster-a")
	}

	if isDocumentReady(doc, "cluster-b") {
		t.Fatal("expected document to be not ready for cluster-b")
	}

	unstructured.SetNestedField(doc.Object, "failed", "status", "status")
	if isDocumentReady(doc, "cluster-a") {
		t.Fatal("expected document to be not ready when status indicates failure")
	}
}

func newDocument(name, namespace, primary, phase string) *unstructured.Unstructured {
	doc := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"clusterReplication": map[string]any{
				"primary": primary,
			},
		},
		"status": map[string]any{
			"status": phase,
		},
	}}
	gvk := schema.GroupVersionKind{Group: documentDBGVRGroup, Version: documentDBGVRVersion, Kind: "DocumentDB"}
	doc.SetGroupVersionKind(gvk)
	doc.SetName(name)
	doc.SetNamespace(namespace)
	return doc
}
