# AKS Fleet Deployment

This directory contains templates for deploying an AKS Fleet with member clusters across different Azure regions with full mesh VNet peering, along with DocumentDB operator and multi-region database deployment capabilities.

## Architecture

- **Fleet Resource**: Deployed in East US 2 (management only)
- **Member Clusters**: Dynamically discovered and deployed across available regions
- **Network**: Full mesh VNet peering between all member clusters
- **VM Size**: Standard_D2ps_v6 (configurable)
- **Node Count**: 1 node per cluster for cost optimization
- **Kubernetes Version**: Uses region default GA version (configurable)
- **DocumentDB**: Multi-region deployment with primary/replica architecture

## Prerequisites

- Azure CLI installed and logged in
- Fleet extension: `az extension install --name fleet`
- Sufficient quota in target regions for AKS clusters
- Contributor access to the subscription
- kubelogin for Azure AD authentication: `az aks install-cli`
- Helm 3.x installed
- jq for JSON processing: `brew install jq` (macOS) or `apt-get install jq` (Linux)

## Deployment

### 1. Deploy AKS Fleet Infrastructure

```bash
./deploy-fleet-bicep.sh
```

The script will:
1. Create a resource group (`german-aks-fleet-rg`)
2. Deploy the Fleet resource
3. Create VNets with non-overlapping IP ranges
4. Deploy member clusters in each region
5. Configure full mesh VNet peering
6. Set up RBAC access for the current user
7. Export FLEET_ID and IDENTITY environment variables
8. Set up kubectl aliases for easy cluster access

### 2. Install cert-manager

Install cert-manager on all member clusters:

```bash
./install-cert-manager.sh
```

This script will:
- Add the jetstack Helm repository
- Install cert-manager with CRDs on each member cluster
- Wait for deployments to be ready
- Display pod status for verification

### 3. Install DocumentDB Operator

Deploy the DocumentDB operator using Fleet:

```bash
./install-documentdb-operator.sh
```

This script will:
- Install cert-manager CRDs on the hub
- Package the local DocumentDB chart
- Deploy the operator via Helm
- Verify deployment across all member clusters
- Show operator pod status on each cluster

### 4. Deploy Multi-Region DocumentDB

Deploy a multi-region DocumentDB cluster with replication:

```bash
# With auto-generated password
./deploy-multi-region.sh

# With custom password
./deploy-multi-region.sh "MySecureP@ssw0rd"

# With environment variable
export DOCUMENTDB_PASSWORD="MySecureP@ssw0rd"
./deploy-multi-region.sh
```

This will:
- **Dynamically discover** all member clusters in the resource group
- **Create cluster identification ConfigMaps** on each cluster for tracking
- **Select a primary cluster** (prefers eastus2, or uses first available)
- **Deploy DocumentDB** with cross-region replication
- **Configure replicas** in all other discovered regions
- **Provide connection information** and failover commands

### Key Features of Multi-Region Deployment

- **Dynamic Cluster Discovery**: No hardcoded cluster names - automatically finds all member clusters
- **Smart Primary Selection**: Automatically selects the best primary cluster
- **Cluster Identification**: Creates ConfigMaps to identify each cluster with name and region
- **Resource Management**: Handles existing resources with options to delete, update, or cancel
- **Generated Failover Commands**: Provides ready-to-use commands for failover to any region

## Configuration

### Fleet Configuration

Edit `parameters.bicepparam` to customize:
- Hub cluster name (used for fleet naming)
- Hub region (fleet location)
- Member regions
- VM sizes
- Node counts
- Kubernetes version

### DocumentDB Configuration

Edit `multi-region.yaml` to customize:
- Database size and instances
- Primary region selection (handled dynamically by script)
- Replication settings
- Service exposure type
- Log levels

The template uses placeholders that are replaced at runtime:
- `${DOCUMENTDB_PASSWORD}`: The database password
- `${PRIMARY_CLUSTER}`: The selected primary cluster
- `${CLUSTER_LIST}`: The list of all discovered clusters

### Network Configuration

Default VNet ranges:
- West US 3 VNet: 10.1.0.0/16
- UK South VNet: 10.2.0.0/16
- East US 2 VNet: 10.3.0.0/16

## Environment Variables

The deployment scripts automatically set and export:
- `FLEET_ID`: Full resource ID of the fleet
- `IDENTITY`: Your Azure AD user ID
- `DOCUMENTDB_PASSWORD`: Database password (when deploying DocumentDB)
- `RESOURCE_GROUP`: Resource group name (default: german-aks-fleet-rg)

## kubectl Aliases

After deployment, these aliases are available:
- `k-hub`: Access to the hub (or first member if fleet hub unavailable)
- `k-westus3`: Access to West US 3 member cluster
- `k-uksouth`: Access to UK South member cluster
- `k-eastus2`: Access to East US 2 member cluster

Load aliases:
```bash
source ~/.bashrc
```

## Fleet Management

```bash
# Show fleet details
az fleet show --name aks-fleet-hub-fleet --resource-group german-aks-fleet-rg

# List fleet members
az fleet member list --fleet-name aks-fleet-hub-fleet --resource-group german-aks-fleet-rg

# Check ClusterResourcePlacement status
kubectl --context hub get clusterresourceplacement

# View placement details
kubectl --context hub describe clusterresourceplacement documentdb-crp
```

## DocumentDB Management

### Check Deployment Status

```bash
# Quick status across all clusters (auto-generated command)
for c in member-eastus2-xxx member-uksouth-xxx member-westus3-xxx; do 
  echo "=== $c ==="
  kubectl --context $c get documentdb,pods -n documentdb-preview-ns 2>/dev/null || echo 'Not deployed yet'
  echo
done

# Check operator status on all clusters
for cluster in $(az aks list -g german-aks-fleet-rg -o json | jq -r '.[] | select(.name|startswith("member-")) | .name'); do
  echo "=== $cluster ==="
  kubectl --context $cluster get deploy -n documentdb-operator
  kubectl --context $cluster get pods -n documentdb-operator
done
```

### Connect to Database

```bash
# Port forward to primary (dynamically determined)
kubectl --context <primary-cluster> port-forward \
  -n documentdb-preview-ns svc/documentdb-preview 10260:10260

# Get connection string (auto-displayed after deployment)
kubectl --context <primary-cluster> get documentdb -n documentdb-preview-ns -A -o json | \
  jq ".items[0].status.connectionString"
```

### Failover Operations

The deployment script generates failover commands for each region:

```bash
# Example: Failover to westus3
kubectl --context hub patch documentdb documentdb-preview -n documentdb-preview-ns \
  --type='merge' -p '{"spec":{"clusterReplication":{"primary":"member-westus3-xxx"}}}'
```

### Monitor Replication

```bash
# Watch ClusterResourcePlacement status
watch 'kubectl --context hub get clusterresourceplacement documentdb-crp -o wide'

# Monitor all DocumentDB instances
watch 'for c in $(az aks list -g german-aks-fleet-rg -o json | jq -r ".[] | select(.name|startswith(\"member-\")) | .name"); do \
  echo "=== $c ==="; \
  kubectl --context $c get documentdb,pods -n documentdb-preview-ns; \
  echo; \
done'
```

## RBAC Management

The deployment script automatically assigns the "Azure Kubernetes Fleet Manager RBAC Cluster Admin" role. To manage RBAC:

```bash
# View current role assignment
az role assignment list --assignee $IDENTITY --scope $FLEET_ID

# Add another user
az role assignment create --role "Azure Kubernetes Fleet Manager RBAC Cluster Admin" \
  --assignee <user-id> --scope $FLEET_ID
```

## Network Verification

Test connectivity between clusters:

```bash
# Deploy a test pod in one cluster
kubectl --context member-westus3-xxx run test-pod --image=nicolaka/netshoot -it --rm -- /bin/bash

# From within the pod, ping services in other clusters
```

Verify VNet peering:

```bash
az network vnet peering list --resource-group german-aks-fleet-rg \
  --vnet-name member-westus3-vnet --output table
```

## Troubleshooting

### Authentication Issues

If you encounter authentication issues:

```bash
# For web-based authentication (default)
az fleet get-credentials --resource-group german-aks-fleet-rg --name aks-fleet-hub-fleet

# If web authentication is blocked, switch to Azure CLI authentication
kubelogin convert-kubeconfig -l azurecli

# For member clusters, use admin credentials
az aks get-credentials --resource-group german-aks-fleet-rg \
  --name <cluster-name> --admin --overwrite-existing
```

### DocumentDB Operator Crashes

If the operator crashes with "exec format error":
1. Check the image architecture matches your nodes (amd64)
2. Review `operator/documentdb-helm-chart/values.yaml` for correct image settings
3. Ensure `forceArch: amd64` is set in values.yaml

### Fleet Resource Propagation

If resources aren't propagating to member clusters:

```bash
# Check ClusterResourcePlacement status
kubectl --context hub get clusterresourceplacement documentdb-crp -o yaml

# Check for placement conditions
kubectl --context hub describe clusterresourceplacement documentdb-crp

# Verify member clusters are joined
az fleet member list --fleet-name aks-fleet-hub-fleet \
  --resource-group german-aks-fleet-rg -o table
```

### Cluster List Formatting Issues

If the cluster list in the YAML is not formatted correctly:
1. Check that `jq` is installed: `which jq`
2. Verify clusters are discovered: `az aks list -g german-aks-fleet-rg -o json | jq -r '.[] | select(.name|startswith("member-")) | .name'`
3. Review the generated YAML preview shown during deployment

### Debugging

Use different kubectl verbosity levels:

```bash
kubectl get pods --v=4  # Debug level
kubectl get pods --v=6  # Shows API requests
kubectl get pods --v=8  # Detailed API troubleshooting
```

## Clean Up

To delete all resources:

```bash
# Delete DocumentDB resources first
kubectl --context hub delete -f multi-region.yaml

# Delete the entire resource group
az group delete --name german-aks-fleet-rg --yes --no-wait
```

## Scripts in this Directory

- `deploy-fleet-bicep.sh`: Main fleet deployment script
- `install-cert-manager.sh`: Installs cert-manager on all member clusters
- `install-documentdb-operator.sh`: Deploys DocumentDB operator via Fleet
- `deploy-multi-region.sh`: Deploys multi-region DocumentDB with dynamic cluster discovery
- `main.bicep`: Bicep template for fleet and cluster deployment
- `parameters.bicepparam`: Parameter file for Bicep deployment
- `multi-region.yaml`: Multi-region DocumentDB configuration template with placeholders

## Key Improvements in Latest Version

1. **Dynamic Cluster Discovery**: No hardcoded cluster names - automatically discovers all deployed clusters
2. **Cluster Identification**: Creates ConfigMaps to track cluster identity and region
3. **Smart Primary Selection**: Automatically selects optimal primary cluster
4. **Resource Conflict Handling**: Detects existing resources and provides options
5. **Generated Commands**: Produces ready-to-use failover and monitoring commands
6. **Better Error Handling**: Improved validation and error messages
7. **Connection String Display**: Shows actual DocumentDB connection string from status

## Additional Resources

- [Azure AKS Fleet Documentation](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/)
- [AKS Authentication Guide](https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication)
- [Fleet ClusterResourcePlacement API](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/concepts-resource-propagation)
- [DocumentDB Kubernetes Operator Documentation](../../README.md)