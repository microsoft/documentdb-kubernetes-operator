package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
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

type fakeDynamicClient struct {
	mu      sync.RWMutex
	objects map[string]*unstructured.Unstructured
}

func newFakeDynamicClient(objs ...*unstructured.Unstructured) dynamic.Interface {
	c := &fakeDynamicClient{objects: map[string]*unstructured.Unstructured{}}
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		key := namespacedName(obj.GetNamespace(), obj.GetName())
		c.objects[key] = obj.DeepCopy()
	}
	return c
}

func (c *fakeDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeNamespaceableResource{client: c, gvr: resource}
}

type fakeNamespaceableResource struct {
	client *fakeDynamicClient
	gvr    schema.GroupVersionResource
}

func (r *fakeNamespaceableResource) Namespace(ns string) dynamic.ResourceInterface {
	return &fakeResource{client: r.client, gvr: r.gvr, namespace: ns}
}

func (r *fakeNamespaceableResource) Create(context.Context, *unstructured.Unstructured, metav1.CreateOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Update(context.Context, *unstructured.Unstructured, metav1.UpdateOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) UpdateStatus(context.Context, *unstructured.Unstructured, metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Delete(context.Context, string, metav1.DeleteOptions, ...string) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) DeleteCollection(context.Context, metav1.DeleteOptions, metav1.ListOptions) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Get(context.Context, string, metav1.GetOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) List(context.Context, metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Watch(context.Context, metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) Apply(context.Context, string, *unstructured.Unstructured, metav1.ApplyOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) ApplyStatus(context.Context, string, *unstructured.Unstructured, metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) DeleteSubresource(context.Context, string, string, metav1.DeleteOptions, ...string) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) GetSubresource(context.Context, string, string, metav1.GetOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) UpdateSubresource(context.Context, string, string, *unstructured.Unstructured, metav1.UpdateOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeNamespaceableResource) PatchSubresource(context.Context, string, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

type fakeResource struct {
	client    *fakeDynamicClient
	gvr       schema.GroupVersionResource
	namespace string
}

func (r *fakeResource) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(subresources) != 0 {
		return nil, fmt.Errorf("not implemented")
	}
	if obj == nil {
		return nil, fmt.Errorf("nil object")
	}
	name := obj.GetName()
	if name == "" {
		return nil, fmt.Errorf("missing name")
	}
	key := namespacedName(r.namespace, name)

	r.client.mu.Lock()
	defer r.client.mu.Unlock()
	r.client.objects[key] = obj.DeepCopy()
	return obj.DeepCopy(), nil
}

func (r *fakeResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeResource) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeResource) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(subresources) != 0 {
		return nil, fmt.Errorf("not implemented")
	}
	key := namespacedName(r.namespace, name)

	r.client.mu.RLock()
	defer r.client.mu.RUnlock()
	obj, ok := r.client.objects[key]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: r.gvr.Group, Resource: r.gvr.Resource}, name)
	}
	return obj.DeepCopy(), nil
}

func (r *fakeResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(subresources) != 0 {
		return nil, fmt.Errorf("not implemented")
	}
	if pt != types.MergePatchType {
		return nil, fmt.Errorf("unsupported patch type %s", pt)
	}

	var patch map[string]any
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, fmt.Errorf("unmarshal patch: %w", err)
	}

	key := namespacedName(r.namespace, name)

	r.client.mu.Lock()
	defer r.client.mu.Unlock()
	obj, ok := r.client.objects[key]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: r.gvr.Group, Resource: r.gvr.Resource}, name)
	}

	mergeMaps(obj.Object, patch)
	r.client.objects[key] = obj
	return obj.DeepCopy(), nil
}

func (r *fakeResource) Apply(context.Context, string, *unstructured.Unstructured, metav1.ApplyOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) ApplyStatus(context.Context, string, *unstructured.Unstructured, metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) DeleteSubresource(context.Context, string, string, metav1.DeleteOptions, ...string) error {
	return fmt.Errorf("not implemented")
}

func (r *fakeResource) GetSubresource(context.Context, string, string, metav1.GetOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) UpdateSubresource(context.Context, string, string, *unstructured.Unstructured, metav1.UpdateOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *fakeResource) PatchSubresource(context.Context, string, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func namespacedName(namespace, name string) string {
	return namespace + "/" + name
}

func mergeMaps(dst map[string]any, patch map[string]any) {
	for k, v := range patch {
		patchMap, ok := v.(map[string]any)
		if !ok {
			dst[k] = v
			continue
		}
		current, ok := dst[k]
		if !ok {
			dst[k] = patchMap
			continue
		}
		currentMap, ok := current.(map[string]any)
		if !ok {
			dst[k] = patchMap
			continue
		}
		mergeMaps(currentMap, patchMap)
	}
}
