// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cnpg

import (
	"cmp"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	util "github.com/microsoft/documentdb-operator/internal/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetCnpgClusterSpec(req ctrl.Request, documentdb *dbpreview.DocumentDB, documentdb_image string, serviceAccountName string, log logr.Logger) *cnpgv1.Cluster {
	sidecarPluginName := documentdb.Spec.SidecarInjectorPluginName
	if sidecarPluginName == "" {
		sidecarPluginName = util.DEFAULT_SIDECAR_INJECTOR_PLUGIN
	}

	// Get the gateway image for this DocumentDB instance
	gatewayImage := util.GetGatewayImageForDocumentDB(documentdb)
	log.Info("Creating CNPG cluster with gateway image", "gatewayImage", gatewayImage, "documentdbName", documentdb.Name, "specGatewayImage", documentdb.Spec.GatewayImage)

	credentialSecretName := documentdb.Spec.DocumentDbCredentialSecret
	if credentialSecretName == "" {
		credentialSecretName = util.DEFAULT_DOCUMENTDB_CREDENTIALS_SECRET
	}

	// Configure storage class - use specified storage class or nil for default
	var storageClass *string
	if documentdb.Spec.Resource.Storage.StorageClass != "" {
		storageClass = &documentdb.Spec.Resource.Storage.StorageClass
	}

	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         documentdb.APIVersion,
					Kind:               documentdb.Kind,
					Name:               documentdb.Name,
					UID:                documentdb.UID,
					Controller:         &[]bool{true}[0], // This cluster is controlled by the DocumentDB instance
					BlockOwnerDeletion: &[]bool{true}[0], // Block DocumentDB deletion until cluster is deleted
				},
			},
		},
		Spec: func() cnpgv1.ClusterSpec {
			spec := cnpgv1.ClusterSpec{
				Instances: documentdb.Spec.InstancesPerNode,
				ImageName: documentdb_image,
				StorageConfiguration: cnpgv1.StorageConfiguration{
					StorageClass: storageClass, // Use configured storage class or default
					Size:         documentdb.Spec.Resource.Storage.PvcSize,
				},
				InheritedMetadata: getInheritedMetadataLabels(documentdb.Name),
				Plugins: []cnpgv1.PluginConfiguration{
					{
						Name: sidecarPluginName,
						Parameters: map[string]string{
							"gatewayImage":               gatewayImage,
							"documentDbCredentialSecret": credentialSecretName,
						},
					},
				},
				PostgresUID: 105,
				PostgresGID: 108,
				PostgresConfiguration: cnpgv1.PostgresConfiguration{
					AdditionalLibraries: []string{"pg_cron", "pg_documentdb_core", "pg_documentdb"},
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
				Bootstrap: getBootstrapConfiguration(documentdb, log),
				LogLevel:  cmp.Or(documentdb.Spec.LogLevel, "info"),
				Backup: &cnpgv1.BackupConfiguration{
					VolumeSnapshot: &cnpgv1.VolumeSnapshotConfiguration{
						SnapshotOwnerReference: "backup", // Set owner reference to 'backup' so that snapshots are deleted when Backup resource is deleted
					},
					Target: cnpgv1.BackupTarget("primary"),
				},
			}
			spec.MaxStopDelay = getMaxStopDelayOrDefault(documentdb)
			return spec
		}(),
	}
}

func getInheritedMetadataLabels(appName string) *cnpgv1.EmbeddedObjectMetadata {
	return &cnpgv1.EmbeddedObjectMetadata{
		Labels: map[string]string{
			util.LABEL_APP:          appName,
			util.LABEL_REPLICA_TYPE: "primary", // TODO: Replace with CNPG default setup
		},
	}
}

func getBootstrapConfiguration(documentdb *dbpreview.DocumentDB, log logr.Logger) *cnpgv1.BootstrapConfiguration {
	if documentdb.Spec.Bootstrap != nil && documentdb.Spec.Bootstrap.Recovery != nil && documentdb.Spec.Bootstrap.Recovery.Backup.Name != "" {
		backupName := documentdb.Spec.Bootstrap.Recovery.Backup.Name
		log.Info("DocumentDB cluster will be bootstrapped from backup", "backupName", backupName)
		return &cnpgv1.BootstrapConfiguration{
			Recovery: &cnpgv1.BootstrapRecovery{
				Backup: &cnpgv1.BackupSource{
					LocalObjectReference: cnpgv1.LocalObjectReference{Name: backupName},
				},
			},
		}
	}

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

// getMaxStopDelayOrDefault returns StopDelay if set, otherwise util.CNPG_DEFAULT_STOP_DELAY
func getMaxStopDelayOrDefault(documentdb *dbpreview.DocumentDB) int32 {
	if documentdb.Spec.Timeouts.StopDelay != 0 {
		return documentdb.Spec.Timeouts.StopDelay
	}
	return util.CNPG_DEFAULT_STOP_DELAY
}
