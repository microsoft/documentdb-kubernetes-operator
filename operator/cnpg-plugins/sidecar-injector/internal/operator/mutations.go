// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package operator

import (
	"context"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	"github.com/cloudnative-pg/cnpg-i/pkg/operator"

	"github.com/documentdb/cnpg-i-sidecar-injector/internal/config"
	"github.com/documentdb/cnpg-i-sidecar-injector/pkg/metadata"
)

// MutateCluster is called to mutate a cluster with the defaulting webhook.
// This function is defaulting the "imagePullPolicy" plugin parameter
func (Implementation) MutateCluster(
	_ context.Context,
	request *operator.OperatorMutateClusterRequest,
) (*operator.OperatorMutateClusterResult, error) {
	cluster, err := decoder.DecodeClusterLenient(request.GetDefinition())
	if err != nil {
		return nil, err
	}

	helper := common.NewPlugin(
		*cluster,
		metadata.PluginName,
	)

	config, valErrs := config.FromParameters(helper)
	if len(valErrs) > 0 {
		return nil, valErrs[0]
	}

	mutatedCluster := cluster.DeepCopy()
	for i := range mutatedCluster.Spec.Plugins {
		if mutatedCluster.Spec.Plugins[i].Name != metadata.PluginName {
			continue
		}

		if mutatedCluster.Spec.Plugins[i].Parameters == nil {
			mutatedCluster.Spec.Plugins[i].Parameters = make(map[string]string)
		}

		mutatedCluster.Spec.Plugins[i].Parameters, err = config.ToParameters()
		if err != nil {
			return nil, err
		}
	}

	patch, err := object.CreatePatch(cluster, mutatedCluster)
	if err != nil {
		return nil, err
	}

	return &operator.OperatorMutateClusterResult{
		JsonPatch: patch,
	}, nil
}
