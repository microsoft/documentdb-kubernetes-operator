# AKS Fleet Deployment

This directory contains simplified templates for deploying an AKS Fleet with member clusters across different Azure regions, along with DocumentDB operator and multi-region database deployment capabilities.

## Architecture

- **Fleet Resource**: Deployed in East US 2 (management only)
- **Member Clusters**: Dynamically discovered and deployed across available regions using default Azure networking
- **Network**: Uses default Azure CNI (no custom VNets or peering required)
- **VM Size**: Standard_DS2_v2 (configurable)
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
- openssl for password generation

## Quick Start

### Deploy Everything (One Command)

```bash
./deploy.sh
```

This single script will:
1. Create resource group
2. Deploy Fleet resource
3. Deploy member clusters in all regions
4. Set up RBAC access
5. Install cert-manager on all clusters
6. Deploy DocumentDB operator
7. Configure kubectl aliases
8. Export environment variables

### Deploy DocumentDB

After the infrastructure is deployed:

```bash
# With auto-generated password
./deploy-documentdb.sh

# With custom password
./deploy-documentdb.sh "MySecureP@ssw0rd"
```

This will:
- Dynamically discover all member clusters
- Create cluster identification ConfigMaps
- Select a primary cluster (prefers eastus2)
- Deploy DocumentDB with cross-region replication
- Provide connection information and failover commands

## Configuration

### Fleet Configuration

Edit `parameters.bicepparam` to customize:
- Hub cluster name (used for fleet naming)
- Hub region (fleet location)
- Member regions
- VM sizes
- Node counts
- Kubernetes version

Or use environment variables:
```bash
export RESOURCE_GROUP="my-fleet-rg"
export HUB_REGION="westus2"
export MEMBER_REGIONS_CSV="westus3,eastus2,centralus"
export HUB_VM_SIZE="Standard_D4s_v3"
./deploy.sh
```

### DocumentDB Configuration

Edit `multi-region.yaml` to customize:
- Database size and instances
- Replication settings
- Service exposure type
- Log levels

The template uses placeholders replaced at runtime:
- `{{DOCUMENTDB_PASSWORD}}`: The database password
- `{{PRIMARY_CLUSTER}}`: The selected primary cluster
- `{{CLUSTER_LIST}}`: The list of all discovered clusters

## Environment Variables

The deployment scripts automatically set and export:
- `FLEET_ID`: Full resource ID of the fleet
- `IDENTITY`: Your Azure AD user ID
- `DOCUMENTDB_PASSWORD`: Database password (when deploying DocumentDB)
- `RESOURCE_GROUP`: Resource group name (default: german-aks-fleet-rg)

## kubectl Aliases

After deployment, aliases are automatically created:
- `k-hub`: Access to the hub (or first member if unavailable)
- `k-<region>`: Access to specific region (e.g., `k-westus3`, `k-eastus2`)

Load aliases:
```bash
source ~/.bashrc
```

## Management

### Check Deployment Status

```bash
# Check operator status
kubectl --context hub get deploy -n documentdb-operator

# Check DocumentDB instances
kubectl --context hub get clusterresourceplacement documentdb-crp -o wide

# View specific cluster
kubectl --context <cluster-name> get documentdb,pods -n documentdb-preview-ns
```

### Connect to Database

Connection information is displayed after deployment. Use port-forward:

```bash
kubectl --context <primary-cluster> port-forward \
  -n documentdb-preview-ns svc/documentdb-preview 10260:10260

mongosh localhost:10260 -u default_user -p <password> \
  --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
```

### Failover Operations

Failover commands are generated during deployment. General syntax:

```bash
kubectl --context hub patch documentdb documentdb-preview \
  -n documentdb-preview-ns \
  --type='merge' \
  -p '{"spec":{"clusterReplication":{"primary":"<new-primary-cluster>"}}}'
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

```bash
# Get fleet credentials
az fleet get-credentials --resource-group $RESOURCE_GROUP --name <fleet-name>

# If web authentication is blocked, use Azure CLI
kubelogin convert-kubeconfig -l azurecli

# Use admin credentials for member clusters
az aks get-credentials --resource-group $RESOURCE_GROUP --name <cluster-name> --admin
```

### Resource Propagation Issues

```bash
# Check ClusterResourcePlacement status
kubectl --context hub get clusterresourceplacement documentdb-crp -o yaml

# Verify fleet members
az fleet member list --fleet-name <fleet-name> --resource-group $RESOURCE_GROUP
```

### Debugging

```bash
# Check operator logs
kubectl --context hub logs -n documentdb-operator deployment/documentdb-operator

# View DocumentDB status
kubectl --context <cluster> describe documentdb -n documentdb-preview-ns
```

## Clean Up

```bash
# Delete DocumentDB resources
kubectl --context hub delete clusterresourceplacement documentdb-crp
kubectl --context hub delete namespace documentdb-preview-ns

# Delete entire resource group
az group delete --name $RESOURCE_GROUP --yes --no-wait
```

## Scripts

- **`deploy.sh`**: All-in-one deployment (Fleet + cert-manager + operator)
- **`deploy-documentdb.sh`**: Deploy multi-region DocumentDB
- **`main.bicep`**: Bicep template for Fleet and clusters (no VNets/peering)
- **`parameters.bicepparam`**: Configuration parameters
- **`multi-region.yaml`**: DocumentDB configuration template

## Key Features

- **Simplified Networking**: Uses default Azure CNI (no custom VNets or peering)
- **Consolidated Scripts**: Two main scripts instead of five
- **Dynamic Discovery**: Automatically finds and configures all clusters
- **Smart Defaults**: Sensible defaults with environment variable overrides
- **Minimal Configuration**: No network planning required

## Additional Resources

- [Azure AKS Fleet Documentation](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/)
- [AKS Authentication Guide](https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication)
- [Fleet ClusterResourcePlacement API](https://learn.microsoft.com/en-us/azure/kubernetes-fleet/concepts-resource-propagation)
- [DocumentDB Kubernetes Operator Documentation](../../README.md)