// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package lifecycle implements the lifecycle hooks
package lifecycle

import (
	"context"
	"errors"
	"log"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	"github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/documentdb/cnpg-i-sidecar-injector/internal/config"
	"github.com/documentdb/cnpg-i-sidecar-injector/internal/utils"
	"github.com/documentdb/cnpg-i-sidecar-injector/pkg/metadata"
)

// Implementation is the implementation of the lifecycle handler
type Implementation struct {
	lifecycle.UnimplementedOperatorLifecycleServer
}

// GetCapabilities exposes the lifecycle capabilities
func (impl Implementation) GetCapabilities(
	_ context.Context,
	_ *lifecycle.OperatorLifecycleCapabilitiesRequest,
) (*lifecycle.OperatorLifecycleCapabilitiesResponse, error) {
	return &lifecycle.OperatorLifecycleCapabilitiesResponse{
		LifecycleCapabilities: []*lifecycle.OperatorLifecycleCapabilities{
			{
				Group: "",
				Kind:  "Pod",
				OperationTypes: []*lifecycle.OperatorOperationType{
					{
						Type: lifecycle.OperatorOperationType_TYPE_CREATE,
					},
					{
						Type: lifecycle.OperatorOperationType_TYPE_PATCH,
					},
				},
			},
		},
	}, nil
}

// LifecycleHook is called when creating Kubernetes services
func (impl Implementation) LifecycleHook(
	ctx context.Context,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	kind, err := utils.GetKind(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}
	operation := request.GetOperationType().GetType().Enum()
	if operation == nil {
		return nil, errors.New("no operation set")
	}

	//nolint: gocritic
	switch kind {
	case "Pod":
		switch *operation {
		case lifecycle.OperatorOperationType_TYPE_CREATE, lifecycle.OperatorOperationType_TYPE_PATCH,
			lifecycle.OperatorOperationType_TYPE_UPDATE:
			return impl.reconcileMetadata(ctx, request)
		}
		// add any other custom logic to execute based on the operation
	}

	return &lifecycle.OperatorLifecycleResponse{}, nil
}

// LifecycleHook is called when creating Kubernetes services
func (impl Implementation) reconcileMetadata(
	ctx context.Context,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	cluster, err := decoder.DecodeClusterLenient(request.GetClusterDefinition())
	if err != nil {
		return nil, err
	}

	// Initialize standard logger for debugging
	log.SetPrefix("[DocumentDB Sidecar Injector] ")

	// Debug: Log the full cluster definition to see what plugins are configured
	log.Printf("Cluster plugins configuration: %+v", cluster.Spec.Plugins)

	helper := common.NewPlugin(
		*cluster,
		metadata.PluginName,
	)

	// Debug logging for plugin parameters and metadata
	log.Printf("Plugin name being used: %s", metadata.PluginName)
	log.Printf("Plugin parameters received: %v, cluster: %s/%s",
		helper.Parameters, cluster.Namespace, cluster.Name)

	// Debug: Check if our plugin is found in the cluster's plugin list
	for i, plugin := range cluster.Spec.Plugins {
		log.Printf("Plugin[%d]: Name=%s, Enabled=%t, Parameters=%v",
			i, plugin.Name, plugin.Enabled, plugin.Parameters)
	}

	configuration, valErrs := config.FromParameters(helper)
	if len(valErrs) > 0 {
		return nil, valErrs[0]
	}

	// Log the gateway image being used
	gatewayImageParam := helper.Parameters["gatewayImage"]
	if gatewayImageParam == "" {
		log.Printf("Using default gateway image: %s (no gatewayImage parameter provided)", configuration.GatewayImage)
	} else {
		log.Printf("Using configured gateway image: %s (parameter value: %s)", configuration.GatewayImage, gatewayImageParam)
	}

	pod, err := decoder.DecodePodJSON(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}

	mutatedPod := pod.DeepCopy()

	// Initialize environment variables
	envVars := []corev1.EnvVar{
		{
			Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
			Value: "http://localhost:4412",
		},
	}

	// Add USERNAME and PASSWORD environment variables from secret defined in configuration
	credentialSecretName := configuration.DocumentDbCredentialSecret
	log.Printf("Adding USERNAME and PASSWORD environment variables from secret '%s'", credentialSecretName)
	envVars = append(envVars,
		corev1.EnvVar{
			Name: "USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: credentialSecretName,
					},
					Key: "username",
				},
			},
		},
		corev1.EnvVar{
			Name: "PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: credentialSecretName,
					},
					Key: "password",
				},
			},
		},
	)

	// Initialize the sidecar container with configurable gateway image
	sidecar := &corev1.Container{
		Name:            "documentdb-gateway",
		Image:           configuration.GatewayImage,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 10260,
			},
		},
		Env: envVars,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  pointer.Int64(1000),
			RunAsGroup: pointer.Int64(1000),
		},
	}

	// Check if the pod has the label replication_cluster_type=replica or is not a primary by CNPG instanceRole
	instanceRole := mutatedPod.Labels["cnpg.io/instanceRole"]
	isLocalReplica := instanceRole == "replica"

	if mutatedPod.Labels["replication_cluster_type"] == "replica" || isLocalReplica {
		sidecar.Args = []string{"--create-user", "false", "--start-pg", "false", "--pg-port", "5432"}
	} else {
		sidecar.Args = []string{"--create-user", "true", "--start-pg", "false", "--pg-port", "5432"}
	}

	// Inject the sidecar container
	err = object.InjectPluginSidecar(mutatedPod, sidecar, false)
	if err != nil {
		return nil, err
	}

	// Apply any custom logic needed here, in this example we just add some metadata to the pod

	for key, value := range configuration.Labels {
		mutatedPod.Labels[key] = value
	}
	for key, value := range configuration.Annotations {
		mutatedPod.Annotations[key] = value
	}

	patch, err := object.CreatePatch(mutatedPod, pod)
	if err != nil {
		return nil, err
	}

	log.Printf("Generated patch: %s", string(patch))

	return &lifecycle.OperatorLifecycleResponse{
		JsonPatch: patch,
	}, nil
}
