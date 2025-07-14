// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package lifecycle implements the lifecycle hooks
package lifecycle

import (
	"context"
	"errors"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	"github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	"github.com/cloudnative-pg/machinery/pkg/log"
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

	logger := log.FromContext(ctx).WithName("cnpg_i_example_lifecyle")
	helper := common.NewPlugin(
		*cluster,
		metadata.PluginName,
	)

	configuration, valErrs := config.FromParameters(helper)
	if len(valErrs) > 0 {
		return nil, valErrs[0]
	}

	pod, err := decoder.DecodePodJSON(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}

	mutatedPod := pod.DeepCopy()

	// Initialize the sidecar container
	sidecar := &corev1.Container{
		Name:            "documentdb-gateway",
		Image:           "ghcr.io/microsoft/documentdb/documentdb-local:16",
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 10260,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "OTEL_EXPORTER_OTLP_ENDPOINT",
				Value: "http://localhost:4412",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  pointer.Int64(1000),
			RunAsGroup: pointer.Int64(1000),
		},
	}

	// Check if the pod has the label replication_cluster_type=replica
	if mutatedPod.Labels["replication_cluster_type"] == "replica" {
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

	logger.Debug("generated patch", "content", string(patch), "configuration", configuration)

	return &lifecycle.OperatorLifecycleResponse{
		JsonPatch: patch,
	}, nil
}
