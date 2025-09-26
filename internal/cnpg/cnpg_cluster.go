// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	util "github.com/microsoft/documentdb-operator/internal/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func GetCnpgClusterSpec(req ctrl.Request, documentdb dbpreview.DocumentDB, documentdb_image string, serviceAccountName string, log logr.Logger) *cnpgv1.Cluster {
	sidecarPluginName := documentdb.Spec.SidecarInjectorPluginName
	if sidecarPluginName == "" {
		sidecarPluginName = util.DEFAULT_SIDECAR_INJECTOR_PLUGIN
	}

	// Get the gateway image for this DocumentDB instance
	gatewayImage := util.GetGatewayImageForDocumentDB(&documentdb)
	log.Info("Creating CNPG cluster with gateway image", "gatewayImage", gatewayImage, "documentdbName", documentdb.Name, "specGatewayImage", documentdb.Spec.GatewayImage)

	credentialSecretName := documentdb.Spec.DocumentDbCredentialSecret
	if credentialSecretName == "" {
		credentialSecretName = util.DEFAULT_DOCUMENTDB_CREDENTIALS_SECRET
	}

	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: func() cnpgv1.ClusterSpec {
			spec := cnpgv1.ClusterSpec{
				Instances: documentdb.Spec.InstancesPerNode,
				ImageName: documentdb_image,
				StorageConfiguration: cnpgv1.StorageConfiguration{
					StorageClass: nil,
					Size:         documentdb.Spec.Resource.PvcSize,
				},
				InheritedMetadata: getInheritedMetadataLabels(documentdb),
				Plugins: func() []cnpgv1.PluginConfiguration {
					params := map[string]string{
						"gatewayImage":               gatewayImage,
						"documentDbCredentialSecret": credentialSecretName,
					}
					if documentdb.Status.TLS != nil && documentdb.Status.TLS.Ready && documentdb.Status.TLS.SecretName != "" {
						params["gatewayTLSSecret"] = documentdb.Status.TLS.SecretName
					}
					return []cnpgv1.PluginConfiguration{{
						Name:       sidecarPluginName,
						Enabled:    pointer.Bool(true),
						Parameters: params,
					}}
				}(),
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
				Bootstrap: getBootstrapConfiguration(documentdb),
			}
			spec.MaxStopDelay = getMaxStopDelayOrDefault(documentdb)
			return spec
		}(),
	}
}

func getInheritedMetadataLabels(documentdb dbpreview.DocumentDB) *cnpgv1.EmbeddedObjectMetadata {
	return &cnpgv1.EmbeddedObjectMetadata{
		Labels: map[string]string{
			util.LABEL_APP:          documentdb.Name,
			util.LABEL_REPLICA_TYPE: "primary", // TODO: Replace with CNPG default setup
		},
	}
}

func getBootstrapConfiguration(documentdb dbpreview.DocumentDB) *cnpgv1.BootstrapConfiguration {
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
func getMaxStopDelayOrDefault(documentdb dbpreview.DocumentDB) int32 {
	if documentdb.Spec.Timeouts.StopDelay != 0 {
		return documentdb.Spec.Timeouts.StopDelay
	}
	return util.CNPG_DEFAULT_STOP_DELAY
}
	}
}

// getMaxStopDelayOrDefault returns StopDelay if set, otherwise util.CNPG_DEFAULT_STOP_DELAY
func getMaxStopDelayOrDefault(documentdb dbpreview.DocumentDB) int32 {
	if documentdb.Spec.Timeouts.StopDelay != 0 {
		return documentdb.Spec.Timeouts.StopDelay
	}
	return util.CNPG_DEFAULT_STOP_DELAY
}
