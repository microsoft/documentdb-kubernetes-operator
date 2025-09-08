// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

const (
	POSTGRES_PORT = "POSTGRES_PORT"
	SIDECAR_PORT  = "SIDECAR_PORT"
	GATEWAY_PORT  = "GATEWAY_PORT"

	COSMOSDB_IMAGE_ENV            = "DOCUMENTDB_IMAGE"
	DOCUMENTDB_SIDECAR_IMAGE_ENV  = "DOCUMENTDB_SIDECAR_IMAGE"
	ENABLE_SCALING_CONTROLLER_ENV = "ENABLE_SCALING_CONTROLLER"

	// DocumentDB versioning environment variable
	DOCUMENTDB_VERSION_ENV = "DOCUMENTDB_VERSION"

	// DocumentDB image repository
	DOCUMENTDB_IMAGE_REPOSITORY = "ghcr.io/microsoft/documentdb/documentdb-local"

	DEFAULT_DOCUMENTDB_IMAGE              = DOCUMENTDB_IMAGE_REPOSITORY + ":16"
	DEFAULT_GATEWAY_IMAGE                 = DOCUMENTDB_IMAGE_REPOSITORY + ":16"
	DEFAULT_DOCUMENTDB_CREDENTIALS_SECRET = "documentdb-credentials"
	DEFAULT_VERSION                       = "v0.1.0"

	LABEL_APP                      = "app"
	LABEL_REPLICA_TYPE             = "replica_type"
	LABEL_NODE_INDEX               = "node_index"
	LABEL_SERVICE_TYPE             = "service_type"
	LABEL_REPLICATION_CLUSTER_TYPE = "replication_cluster_type"

	HEADLESS_INTERNAL_SERVICE_TYPE = "headless-internal-service"

	DOCUMENTDB_SERVICE_PREFIX = "documentdb-service-"

	DEFAULT_SIDECAR_INJECTOR_PLUGIN = "cnpg-i-sidecar-injector.documentdb.io"

	CNPG_DEFAULT_STOP_DELAY = 30
)
