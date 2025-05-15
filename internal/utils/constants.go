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

	DEFAULT_DOCUMENTDB_IMAGE = "ghcr.io/microsoft/documentdb/documentdb-local:latest"

	LABEL_APP                      = "app"
	LABEL_REPLICA_TYPE             = "replica_type"
	LABEL_NODE_INDEX               = "node_index"
	LABEL_SERVICE_TYPE             = "service_type"
	LABEL_REPLICATION_CLUSTER_TYPE = "replication_cluster_type"

	HEADLESS_INTERNAL_SERVICE_TYPE = "headless-internal-service"

	LOADBALANCER_PREFIX = "documentdb-service-"

	DEFAULT_SIDECAR_INJECTOR_PLUGIN = "cnpg-i-hello-world.cloudnative-pg.io"
)
