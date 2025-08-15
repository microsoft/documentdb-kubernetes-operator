# AKS Fleet Deployment

This directory contains templates for deploying an AKS Fleet with member clusters across different Azure regions with full mesh VNet peering, along with DocumentDB operator and multi-region database deployment capabilities.

## Architecture

- **Fleet Resource**: Deployed in East US 2 (management only)
- **Member Clusters**: Deployed in West US 3, UK South, and East US 2
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
- envsubst (usually part of gettext package)

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
- Create a DocumentDB namespace
- Deploy a primary DocumentDB instance in East US 2
- Configure replicas in West US 3 and UK South
- Set up cross-region replication
- Provide connection information

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
- Primary region selection
- Replication settings
- Service exposure type
- Log levels

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
k-hub get clusterresourceplacement

# View placement details
k-hub describe clusterresourceplacement documentdb-crp
```

## DocumentDB Management

### Check Deployment Status

```bash
# Check operator status on all clusters
for cluster in member-{westus3,uksouth,eastus2}-z2fyhq65f4ktg; do
  echo "=== $cluster ==="
  kubectl --context $cluster get deploy -n documentdb-operator
  kubectl --context $cluster get pods -n documentdb-operator
done

# Check DocumentDB instances
for cluster in member-{westus3,uksouth,eastus2}-z2fyhq65f4ktg; do
  echo "=== $cluster ==="
  kubectl --context $cluster get documentdb -n documentdb-preview-ns
done
```

### Connect to Database

```bash
# Port forward to primary (East US 2)
kubectl --context member-eastus2-z2fyhq65f4ktg port-forward \
  -n documentdb-preview-ns svc/documentdb-preview 5432:5432

# In another terminal, connect with psql
psql postgresql://default_user:$DOCUMENTDB_PASSWORD@localhost:5432/documentdb
```

### Monitor Replication

```bash
# Watch all DocumentDB instances
watch 'for c in member-{westus3,uksouth,eastus2}-z2fyhq65f4ktg; do \
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
k-westus3 run test-pod --image=nicolaka/netshoot -it --rm -- /bin/bash

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
2. Review `documentdb-chart/values.yaml` for correct image settings
3. Ensure `forceArch: amd64` is set in values.yaml

### Fleet Resource Propagation

If resources aren't propagating to member clusters:

```bash
# Check ClusterResourcePlacement status
k-hub get clusterresourceplacement documentdb-crp -o yaml

# Check for placement conditions
k-hub describe clusterresourceplacement documentdb-crp

# Verify member clusters are joined
az fleet member list --fleet-name aks-fleet-hub-fleet \
  --resource-group german-aks-fleet-rg -o table
```

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
k-hub delete -f multi-region.yaml

# Delete the entire resource group
az group delete --name german-aks-fleet-rg --yes --no-wait
```

## Scripts in this Directory

- `deploy-fleet-bicep.sh`: Main fleet deployment script
- `install-cert-manager.sh`: Installs cert-manager on all member clusters
- `install-documentdb-operator.sh`: Deploys DocumentDB operator via Fleet
- `deploy-multi-region.sh`: Deploys multi-region DocumentDB with replication
- `main.bicep`: Bicep template for fleet and cluster deployment
- `parameters.bicepparam`: Parameter file for Bicep deployment
- `multi-region.yaml`: Multi-region DocumentDB configuration template

## Additional Resources

- [Azure AKS Fleet Documentation](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/)
- [AKS Authentication Guide](https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication)
- [Fleet ClusterResourcePlacement API](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/concepts-resource-propagation)