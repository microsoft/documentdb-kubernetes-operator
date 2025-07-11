// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

import (
	"context"
	"fmt"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReplicationContext struct {
	Self           string
	Others         []string
	Source         string
	currentPrimary string
	targetPrimary  string
	state          replicationState
}

type replicationState int32

const (
	NoReplication replicationState = iota
	Primary
	Replica
)

func GetReplicationContext(ctx context.Context, client client.Client, documentdb dbpreview.DocumentDB) (*ReplicationContext, error) {
	if documentdb.Spec.ClusterReplication == nil {
		return &ReplicationContext{
			state: NoReplication,
			Self:  documentdb.Name,
		}, nil
	}

	self, others, err := splitSelfAndOthers(ctx, client, documentdb)
	if err != nil {
		return nil, err
	}

	// If no remote clusters, then just proceed with a regular cluster
	if len(others) == 0 {
		return &ReplicationContext{
			state: NoReplication,
			Self:  documentdb.Name,
		}, nil
	}

	state := Replica
	if documentdb.Spec.ClusterReplication.Primary == self {
		state = Primary
	}

	source := documentdb.Spec.ClusterReplication.Primary
	if state == Primary {
		source = others[0]
	}

	return &ReplicationContext{
		Self:           self,
		Others:         others,
		Source:         source,
		state:          state,
		targetPrimary:  documentdb.Status.TargetPrimary,
		currentPrimary: documentdb.Status.LocalPrimary,
	}, nil
}

// String implements fmt.Stringer interface for better logging output
func (r ReplicationContext) String() string {
	stateStr := ""
	switch r.state {
	case NoReplication:
		stateStr = "NoReplication"
	case Primary:
		stateStr = "Primary"
	case Replica:
		stateStr = "Replica"
	}

	return fmt.Sprintf("ReplicationContext{Self: %s, State: %s, Others: %v, Source: %s, CurrentPrimary: %s, TargetPrimary: %s}",
		r.Self, stateStr, r.Others, r.Source, r.currentPrimary, r.targetPrimary)
}

// Returns true if this instance is the primary or if there is no replication configured.
func (r ReplicationContext) IsPrimary() bool {
	return r.state == Primary || r.state == NoReplication
}

func (r *ReplicationContext) IsReplicating() bool {
	return r.state == Replica || r.state == Primary
}

// EndpointEnabled returns true if the endpoint should be enabled for this DocumentDB instance.
// The endpoint is enabled when there is no replication configured or when the current primary
// matches the target primary in a replication setup.
func (r ReplicationContext) EndpointEnabled() bool {
	if r.state == NoReplication {
		return true
	}
	return r.currentPrimary == r.targetPrimary
}

func (r ReplicationContext) GenerateIncomingServiceName(resourceGroup string) string {
	return generateServiceName(r.Source, r.Self, resourceGroup)
}

// Create an iterator that yields outgoing service names, for use in a for each loop
func (r ReplicationContext) GenerateOutgoingServiceNames(resourceGroup string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for _, other := range r.Others {
			serviceName := generateServiceName(r.Self, other, resourceGroup)
			if !yield(serviceName) {
				break
			}
		}
	}
}

func generateServiceName(source, target, resourceGroup string) string {
	name := fmt.Sprintf("%s-%s", source, target)
	diff := 63 - len(name) - len(resourceGroup) - 2
	if diff >= 0 {
		return name
	} else {
		// truncate source and target region names equally if needed
		truncateBy := (-diff + 1) / 2 // +1 to handle odd numbers
		sourceLen := len(source) - truncateBy
		targetLen := len(target) - truncateBy
		return fmt.Sprintf("%s-%s", source[0:sourceLen], target[0:targetLen])
	}
}

// Creates the standby names list, which will be all other clusters in addition to "pg_receivewal"
func (r *ReplicationContext) CreateStandbyNamesList() []string {
	standbyNames := make([]string, len(r.Others)+1)
	copy(standbyNames, r.Others)
	standbyNames[len(r.Others)] = "pg_receivewal"
	return standbyNames
}

func splitSelfAndOthers(ctx context.Context, client client.Client, documentdb dbpreview.DocumentDB) (string, []string, error) {
	self := documentdb.Name
	var err error

	if documentdb.Spec.ClusterReplication.EnableFleetForCrossCloud {
		self, err = GetSelfName(ctx, client)
		if err != nil {
			return "", nil, err
		}
	}

	// Set the source to be the first cluster in the list that isn't self
	others := []string{}
	for _, c := range documentdb.Spec.ClusterReplication.ClusterList {
		if c != self {
			others = append(others, c)
		}
	}
	return self, others, nil
}

func GetSelfName(ctx context.Context, client client.Client) (string, error) {
	clusterMapName := "cluster-name"
	clusterNameConfigMap := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Name: clusterMapName, Namespace: "kube-system"}, clusterNameConfigMap)
	if err != nil {
		return "", err
	}

	self := clusterNameConfigMap.Data["name"]
	if self == "" {
		return "", fmt.Errorf("name key not found in kube-system:cluster-name configmap")
	}
	return self, nil
}
