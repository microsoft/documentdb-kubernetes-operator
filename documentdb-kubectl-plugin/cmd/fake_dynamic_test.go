package cmd

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func documentDBGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: documentDBGVRGroup, Version: documentDBGVRVersion, Resource: documentDBGVRResource}
}

func documentDBGK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: documentDBGVRGroup, Version: documentDBGVRVersion, Kind: "DocumentDB"}
}

func newDocumentScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	gk := documentDBGK()
	scheme.AddKnownTypeWithName(gk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gk.GroupVersion().WithKind("DocumentDBList"), &unstructured.UnstructuredList{})
	return scheme
}

func documentListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{documentDBGVR(): "DocumentDBList"}
}
