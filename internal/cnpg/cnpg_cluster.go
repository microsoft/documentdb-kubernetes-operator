// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License. See LICENSE file in the project root for full license information.

package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbpreview "github.com/azure/documentdb-operator/api/preview"
	util "github.com/azure/documentdb-operator/internal/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetCnpgClusterSpec(req ctrl.Request, documentdb dbpreview.DocumentDB, documentdb_image string, serviceAccountName string, log logr.Logger) *cnpgv1.Cluster {
	cnpgSidecarPluginName := documentdb.Spec.CNPGSidecarPlugin
	if cnpgSidecarPluginName == "" {
		cnpgSidecarPluginName = util.DEFAULT_CNPG_SIDECAR_PLUGIN
	}
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: cnpgv1.ClusterSpec{
			Instances: documentdb.Spec.InstancesPerNode,
			ImageName: documentdb_image,
			StorageConfiguration: cnpgv1.StorageConfiguration{
				StorageClass: nil,
				Size:         documentdb.Spec.Resource.PvcSize,
			},
			InheritedMetadata: GetInheritedMetadataLabels(documentdb),
			Plugins: []cnpgv1.PluginConfiguration{
				{
					Name: cnpgSidecarPluginName,
				},
			},
			PostgresUID: 105,
			PostgresGID: 108,
			PostgresConfiguration: cnpgv1.PostgresConfiguration{
				AdditionalLibraries: []string{
					"citus", "pg_cron", "pg_documentdb_core", "pg_documentdb"},
				Parameters: map[string]string{
					"cron.database_name":    "postgres",
					"max_replication_slots": "10",
					"max_wal_senders":       "10",
				},
				PgHBA: []string{
					"host all all 0.0.0.0/0 trust",
					"host all all ::0/0 trust",
					"host replication all all trust",
				},
			},
			Bootstrap: GetBootstrapConfiguration(documentdb),
		},
	}
}

func GetInheritedMetadataLabels(documentdb dbpreview.DocumentDB) *cnpgv1.EmbeddedObjectMetadata {
	return &cnpgv1.EmbeddedObjectMetadata{
		Labels: map[string]string{
			util.LABEL_APP:          documentdb.Name,
			util.LABEL_REPLICA_TYPE: "primary", // TODO: Replace with CNPG default setup
		},
	}
}

func GetBootstrapConfiguration(documentdb dbpreview.DocumentDB) *cnpgv1.BootstrapConfiguration {
	return &cnpgv1.BootstrapConfiguration{
		InitDB: &cnpgv1.BootstrapInitDB{
			PostInitSQL: []string{
				"CREATE EXTENSION documentdb CASCADE",
				"CREATE ROLE documentdb WITH LOGIN PASSWORD 'Admin100'",
				"ALTER ROLE documentdb WITH SUPERUSER CREATEDB CREATEROLE REPLICATION BYPASSRLS",
			},
		},
	}
}
