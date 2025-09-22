// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

const (
	POSTGRES_PORT = "POSTGRES_PORT"
	SIDECAR_PORT  = "SIDECAR_PORT"
	GATEWAY_PORT  = "GATEWAY_PORT"

	// DocumentDB versioning environment variable
	DOCUMENTDB_VERSION_ENV = "DOCUMENTDB_VERSION"

	// DocumentDB image repository
	DOCUMENTDB_IMAGE_REPOSITORY = "ghcr.io/microsoft/documentdb/documentdb-local"

	DEFAULT_DOCUMENTDB_IMAGE              = DOCUMENTDB_IMAGE_REPOSITORY + ":16"
	DEFAULT_GATEWAY_IMAGE                 = DOCUMENTDB_IMAGE_REPOSITORY + ":16"
	DEFAULT_DOCUMENTDB_CREDENTIALS_SECRET = "documentdb-credentials"

	LABEL_APP                      = "app"
	LABEL_REPLICA_TYPE             = "replica_type"
	LABEL_NODE_INDEX               = "node_index"
	LABEL_SERVICE_TYPE             = "service_type"
	LABEL_REPLICATION_CLUSTER_TYPE = "replication_cluster_type"

	DOCUMENTDB_SERVICE_PREFIX = "documentdb-service-"

	DEFAULT_SIDECAR_INJECTOR_PLUGIN = "cnpg-i-sidecar-injector.documentdb.io"

	CNPG_DEFAULT_STOP_DELAY = 30
)
