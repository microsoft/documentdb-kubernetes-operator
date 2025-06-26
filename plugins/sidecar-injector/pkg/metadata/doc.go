// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package metadata contains the metadata of this plugin
package metadata

import "github.com/cloudnative-pg/cnpg-i/pkg/identity"

// PluginName is the name of the plugin
const PluginName = "cnpg-i-sidecar-injector.documentdb.io"

// Data is the metadata of this plugin
var Data = identity.GetPluginMetadataResponse{
	Name:          PluginName,
	Version:       "0.0.1",
	DisplayName:   "Document DB Gateway Sidecar Injector",
	ProjectUrl:    "https://github.com/documentdb/cnpg-i-sidecar-injector",
	RepositoryUrl: "https://github.com/documentdb/cnpg-i-sidecar-injector",
	License:       "Proprietary",
	LicenseUrl:    "https://github.com/documentdb/cnpg-i-sidecar-injector/LICENSE",
	Maturity:      "alpha",
}
