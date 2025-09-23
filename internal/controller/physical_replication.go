// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	util "github.com/microsoft/documentdb-operator/internal/utils"
	fleetv1alpha1 "go.goms.io/fleet-networking/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DocumentDBReconciler) AddClusterReplicationToClusterSpec(
	ctx context.Context,
	documentdb *dbpreview.DocumentDB,
	replicationContext *util.ReplicationContext,
	cnpgCluster *cnpgv1.Cluster,
) error {
	isPrimary := documentdb.Spec.ClusterReplication.Primary == replicationContext.Self

	if documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud {
		err := r.CreateServiceImportAndExport(ctx, replicationContext, documentdb)
		if err != nil {
			return err
		}
	}

	// No more errors possible, so we can safely edit the spec
	cnpgCluster.Name = replicationContext.Self

	if !isPrimary {
		cnpgCluster.Spec.InheritedMetadata.Labels[util.LABEL_REPLICATION_CLUSTER_TYPE] = "replica"
		cnpgCluster.Spec.Bootstrap = &cnpgv1.BootstrapConfiguration{
			PgBaseBackup: &cnpgv1.BootstrapPgBaseBackup{
				Source:   documentdb.Spec.ClusterReplication.Primary,
				Database: "postgres",
				Owner:    "postgres",
			},
		}
	} else if documentdb.Spec.ClusterReplication.HighAvailability {
		// If primary and HA we want a local standby and a slot for the WAL replica
		cnpgCluster.Spec.Instances = 2
		cnpgCluster.Spec.Bootstrap.InitDB.PostInitSQL =
			append(cnpgCluster.Spec.Bootstrap.InitDB.PostInitSQL,
				"select * from pg_create_physical_replication_slot('wal_replica');")
		// Also need to configure quorum writes
		cnpgCluster.Spec.PostgresConfiguration.Synchronous = &cnpgv1.SynchronousReplicaConfiguration{
			Method:          cnpgv1.SynchronousReplicaConfigurationMethodAny,
			Number:          3,
			StandbyNamesPre: replicationContext.CreateStandbyNamesList(),
			DataDurability:  cnpgv1.DataDurabilityLevelRequired,
		}
		trueVal := true
		cnpgCluster.Spec.ReplicationSlots = &cnpgv1.ReplicationSlotsConfiguration{
			SynchronizeReplicas: &cnpgv1.SynchronizeReplicasConfiguration{
				Enabled: &trueVal,
			},
		}

		walReplicaPluginName := documentdb.Spec.WalReplicaPluginName
		if walReplicaPluginName == "" {
			walReplicaPluginName = util.DEFAULT_WAL_REPLICA_PLUGIN
		}
		cnpgCluster.Spec.Plugins = append(cnpgCluster.Spec.Plugins,
			cnpgv1.PluginConfiguration{
				Name: walReplicaPluginName,
			})
	}

	cnpgCluster.Spec.ReplicaCluster = &cnpgv1.ReplicaClusterConfiguration{
		Source:  replicationContext.GetReplicationSource(),
		Primary: documentdb.Spec.ClusterReplication.Primary,
		Self:    replicationContext.Self,
	}

	if documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud {
		// need to create services for each of the other clusters
		cnpgCluster.Spec.Managed = &cnpgv1.ManagedConfiguration{
			Services: &cnpgv1.ManagedServices{
				Additional: []cnpgv1.ManagedService{},
			},
		}
		for serviceName := range replicationContext.GenerateOutgoingServiceNames(documentdb.Namespace) {
			cnpgCluster.Spec.Managed.Services.Additional = append(cnpgCluster.Spec.Managed.Services.Additional,
				cnpgv1.ManagedService{
					SelectorType: cnpgv1.ServiceSelectorTypeRW,
					ServiceTemplate: cnpgv1.ServiceTemplateSpec{
						ObjectMeta: cnpgv1.Metadata{
							Name: serviceName,
						},
					},
				})
		}
	}
	selfHost := documentdb.Name + "-rw." + documentdb.Namespace + ".svc"
	cnpgCluster.Spec.ExternalClusters = []cnpgv1.ExternalCluster{
		{
			Name: replicationContext.Self,
			ConnectionParameters: map[string]string{
				"host":   selfHost,
				"port":   "5432",
				"dbname": "postgres",
				"user":   "postgres",
			},
		},
	}
	for clusterName, serviceName := range replicationContext.GenerateExternalClusterServices(documentdb.Namespace, documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud) {
		cnpgCluster.Spec.ExternalClusters = append(cnpgCluster.Spec.ExternalClusters, cnpgv1.ExternalCluster{
			Name: clusterName,
			ConnectionParameters: map[string]string{
				"host":   serviceName,
				"port":   "5432",
				"dbname": "postgres",
				"user":   "postgres",
			},
		})
	}

	return nil
}

func (r *DocumentDBReconciler) CreateServiceImportAndExport(ctx context.Context, replicationContext *util.ReplicationContext, documentdb *dbpreview.DocumentDB) error {
	for serviceName := range replicationContext.GenerateOutgoingServiceNames(documentdb.Namespace) {
		foundServiceExport := &fleetv1alpha1.ServiceExport{}
		err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: documentdb.Namespace}, foundServiceExport)
		if err != nil && errors.IsNotFound(err) {
			log.Log.Info("Service Export not found. Creating a new Service Export " + serviceName)

			// Service Export
			ringServiceExport := &fleetv1alpha1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: documentdb.Namespace,
				},
			}
			err = r.Create(ctx, ringServiceExport)
			if err != nil {
				return err
			}
		}
	}

	// Below is true because this function is only called if we are fleet enabled
	for sourceServiceName := range replicationContext.GenerateIncomingServiceNames(documentdb.Namespace) {
		foundMCS := &fleetv1alpha1.MultiClusterService{}
		err := r.Get(ctx, types.NamespacedName{Name: sourceServiceName, Namespace: documentdb.Namespace}, foundMCS)
		if err != nil && errors.IsNotFound(err) {
			log.Log.Info("Multi Cluster Service not found. Creating a new Multi Cluster Service")
			// Multi Cluster Service
			foundMCS = &fleetv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourceServiceName,
					Namespace: documentdb.Namespace,
				},
				Spec: fleetv1alpha1.MultiClusterServiceSpec{
					ServiceImport: fleetv1alpha1.ServiceImportRef{
						Name: sourceServiceName,
					},
				},
			}
			err = r.Create(ctx, foundMCS)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DocumentDBReconciler) TryUpdateCluster(ctx context.Context, current, desired *cnpgv1.Cluster, documentdb *dbpreview.DocumentDB) (error, time.Duration) {
	if current.Spec.ReplicaCluster == nil || desired.Spec.ReplicaCluster == nil {
		// FOR NOW assume that we aren't going to turn on or off physical replication
		return nil, -1
	}

	// Update the primary if it has changed
	primaryChanged := current.Spec.ReplicaCluster.Primary != desired.Spec.ReplicaCluster.Primary

	tokenNeedsUpdate, err := r.PromotionTokenNeedsUpdate(ctx, current.Namespace)
	if err != nil {
		return err, time.Second * 10
	}

	if current.Spec.ReplicaCluster.Self != desired.Spec.ReplicaCluster.Self {
		return fmt.Errorf("self cannot be changed"), time.Second * 60
	}

	// TODO update the external clusters

	if tokenNeedsUpdate || primaryChanged && current.Spec.ReplicaCluster.Primary == current.Spec.ReplicaCluster.Self {
		// Primary => replica
		// demote
		current.Spec.ReplicaCluster.Primary = desired.Spec.ReplicaCluster.Primary
		err := r.Client.Update(ctx, current)
		if err != nil {
			return err, time.Second * 10
		}

		// push out the  promotion token
		err = r.CreateTokenService(ctx, current.Status.DemotionToken, documentdb.Namespace, documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud)
		if err != nil {
			return err, time.Second * 10
		}
	} else if primaryChanged && current.Spec.ReplicaCluster.Primary != current.Spec.ReplicaCluster.Self {
		// Replica => primary
		// Look for the token
		oldPrimaryAvailable := slices.Contains(
			documentdb.Spec.ClusterReplication.ClusterList,
			current.Spec.ReplicaCluster.Primary)

		// If the old primary is available, we can read the token from it
		if oldPrimaryAvailable {
			token, err, refreshTime := r.ReadToken(ctx, documentdb.Namespace, documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud)
			if err != nil || refreshTime > 0 {
				return err, refreshTime
			}
			log.Log.Info("Token read successfully", "token", token)
			current.Spec.ReplicaCluster.PromotionToken = token
		}

		// If the old primary is not available, just come up
		current.Spec.ReplicaCluster.Primary = desired.Spec.ReplicaCluster.Primary
		err = r.Client.Update(ctx, current)
		if err != nil {
			return err, time.Second * 10
		}
	}

	return nil, -1
}

func (r *DocumentDBReconciler) ReadToken(ctx context.Context, namespace string, fleetEnabled bool) (string, error, time.Duration) {
	tokenServiceName := "promotion-token"

	// If we are not using fleet, we only need to read the token from the configmap
	if !fleetEnabled {
		configMap := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: tokenServiceName, Namespace: namespace}, configMap)
		if err != nil {
			return "", err, time.Second * 10
		}
		if configMap.Data["index.html"] == "" {
			return "", fmt.Errorf("token not found in configmap"), time.Second * 10
		}
		return configMap.Data["index.html"], nil, -1
	}

	foundMCS := &fleetv1alpha1.MultiClusterService{}
	err := r.Get(ctx, types.NamespacedName{Name: tokenServiceName, Namespace: namespace}, foundMCS)
	if err != nil && errors.IsNotFound(err) {
		foundMCS = &fleetv1alpha1.MultiClusterService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tokenServiceName,
				Namespace: namespace,
			},
			Spec: fleetv1alpha1.MultiClusterServiceSpec{
				ServiceImport: fleetv1alpha1.ServiceImportRef{
					Name: tokenServiceName,
				},
			},
		}
		err = r.Create(ctx, foundMCS)
		if err != nil {
			return "", err, time.Second * 10
		}
	} else if err != nil {
		return "", err, time.Second * 10
	}

	tokenRequestUrl := fmt.Sprintf("http://%s-%s.fleet-system.svc", namespace, tokenServiceName)
	resp, err := http.Get(tokenRequestUrl)
	if err != nil {
		return "", fmt.Errorf("failed to get token from service: %w", err), time.Second * 10
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err), time.Second * 10
	}

	// Need to convert byte array to byte slice before converting to string
	return string(token[:]), nil, -1
}

// TODO make this not have to check the configmap twice
// RETURN true if we have a configmap with a blank token
func (r *DocumentDBReconciler) PromotionTokenNeedsUpdate(ctx context.Context, namespace string) (bool, error) {
	tokenServiceName := "promotion-token"
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: tokenServiceName, Namespace: namespace}, configMap)
	if err != nil {
		// If we don't find the map, we don't need to update
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	// Otherwise, we need to update if the value is blank
	return configMap.Data["index.html"] == "", nil
}

func (r *DocumentDBReconciler) CreateTokenService(ctx context.Context, token string, namespace string, fleetEnabled bool) error {
	tokenServiceName := "promotion-token"
	labels := map[string]string{
		"app": tokenServiceName,
	}

	// Create ConfigMap with token and nginx config
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenServiceName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"index.html": token,
		},
	}

	err := r.Client.Create(ctx, configMap)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			configMap.Data["index.html"] = token
			err = r.Client.Update(ctx, configMap)
			if err != nil {
				return fmt.Errorf("failed to update token ConfigMap: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create token ConfigMap: %w", err)
		}
	}

	if token == "" {
		return fmt.Errorf("No token found yet")
	}

	// When not using fleet, just transfer with the configmap
	if !fleetEnabled {
		return nil
	}

	// Create nginx Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:alpine",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      "TCP",
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      tokenServiceName,
							MountPath: "usr/share/nginx/html",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: tokenServiceName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: tokenServiceName,
							},
						},
					},
				},
			},
		},
	}

	err = r.Client.Create(ctx, pod)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create nginx Pod: %w", err)
	}

	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   "TCP",
				},
			},
		},
	}

	err = r.Client.Create(ctx, service)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Service: %w", err)
	}

	// Create ServiceExport for fleet networking
	serviceExport := &fleetv1alpha1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenServiceName,
			Namespace: namespace,
		},
	}

	err = r.Client.Create(ctx, serviceExport)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ServiceExport: %w", err)
	}

	return nil
}
