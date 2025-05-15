// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package preview contains API Schema definitions for the db preview API group.
// +kubebuilder:object:generate=true
// +groupName=db.microsoft.com
package preview

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "db.microsoft.com", Version: "preview"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
