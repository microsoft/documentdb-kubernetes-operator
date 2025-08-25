#!/usr/bin/env bash
# filepath: /Users/geeichbe/Projects/documentdb-kubernetes-operator/scripts/aks-fleet-deployment/deploy-multi-region.sh
set -euo pipefail

# Deploy multi-region DocumentDB using Fleet
# Usage: ./deploy-multi-region.sh [password]

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Define variables (allow env overrides)# Define variables (allow env overrides)
RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"

# Set password from argument or environment variable
DOCUMENTDB_PASSWORD="${1:-${DOCUMENTDB_PASSWORD:-}}"

# If no password provided, generate a secure one
if [ -z "$DOCUMENTDB_PASSWORD" ]; then
  echo "No password provided. Generating a secure password..."
  DOCUMENTDB_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)
  echo "Generated password: $DOCUMENTDB_PASSWORD"
  echo "(Save this password - you'll need it to connect to the database)"
  echo ""
fi

# Export for envsubst
export DOCUMENTDB_PASSWORD

# Create a temporary file with substituted values
TEMP_YAML=$(mktemp)
envsubst < "$SCRIPT_DIR/multi-region.yaml" > "$TEMP_YAML"

# Load environment if available
if [ -f "$HOME/.bashrc" ]; then
  source "$HOME/.bashrc" || true
fi

# Define member clusters
MEMBER_CLUSTERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name')

# Step 1: Create cluster identification ConfigMaps on each member cluster
echo "======================================="
echo "Creating cluster identification ConfigMaps..."
echo "======================================="

for cluster in $MEMBER_CLUSTERS; do
  echo ""
  echo "Creating ConfigMap for $cluster..."
  
  # Check if context exists
  if ! kubectl config get-contexts "$cluster" &>/dev/null; then
    echo "✗ Context $cluster not found, skipping"
    continue
  fi
  
  # Create or update the cluster-name ConfigMap
  kubectl --context "$cluster" create configmap cluster-name \
    -n kube-system \
    --from-literal=name="$cluster" \
    --dry-run=client -o yaml | kubectl --context "$cluster" apply -f -
  
  # Verify the ConfigMap was created
  if kubectl --context "$cluster" get configmap cluster-name -n kube-system &>/dev/null; then
    echo "✓ ConfigMap created/updated for $cluster"
    # Show the cluster name
    CLUSTER_ID=$(kubectl --context "$cluster" get configmap cluster-name -n kube-system -o jsonpath='{.data.name}')
    echo "  Cluster identified as: $CLUSTER_ID"
  else
    echo "✗ Failed to create ConfigMap for $cluster"
  fi
done

# Step 2: Deploy DocumentDB resources via Fleet
echo ""
echo "======================================="
echo "Deploying DocumentDB multi-region configuration..."
echo "======================================="

# Determine hub context
HUB_CONTEXT="${HUB_CONTEXT:-hub}"
if ! kubectl config get-contexts "$HUB_CONTEXT" &>/dev/null; then
  echo "Hub context not found, trying to find first member cluster..."
  HUB_CONTEXT=$(kubectl config get-contexts -o name | grep -E "member-.*-z2fyhq65f4ktg" | head -1)
  if [ -z "$HUB_CONTEXT" ]; then
    echo "Error: No suitable context found. Please ensure you have credentials for the fleet."
    exit 1
  fi
fi

echo "Using hub context: $HUB_CONTEXT"

# Apply the configuration
echo "Applying DocumentDB multi-region configuration..."
kubectl --context "$HUB_CONTEXT" apply -f "$TEMP_YAML"

# Clean up temp file
rm -f "$TEMP_YAML"

# Check the ClusterResourcePlacement status
echo ""
echo "Checking ClusterResourcePlacement status..."
kubectl --context "$HUB_CONTEXT" get clusterresourceplacement documentdb-crp -o wide

# Wait a bit for propagation
echo ""
echo "Waiting for resources to propagate to member clusters..."
sleep 10

# Step 3: Verify deployment on each member cluster
echo ""
echo "======================================="
echo "Checking deployment status on member clusters..."
echo "======================================="

for cluster in $MEMBER_CLUSTERS; do
  echo ""
  echo "=== $cluster ==="
  
  # Check if context exists
  if ! kubectl config get-contexts "$cluster" &>/dev/null; then
    echo "✗ Context not found, skipping"
    continue
  fi
  
  # Check ConfigMap
  if kubectl --context "$cluster" get configmap cluster-name -n kube-system &>/dev/null; then
    CLUSTER_ID=$(kubectl --context "$cluster" get configmap cluster-name -n kube-system -o jsonpath='{.data.name}')
    echo "✓ Cluster identified as: $CLUSTER_ID"
  else
    echo "✗ Cluster identification ConfigMap not found"
  fi
  
  # Check if namespace exists
  if kubectl --context "$cluster" get namespace documentdb-preview-ns &>/dev/null; then
    echo "✓ Namespace exists"
    
    # Check if secret exists
    if kubectl --context "$cluster" get secret documentdb-credentials -n documentdb-preview-ns &>/dev/null; then
      echo "✓ Secret exists"
    else
      echo "✗ Secret not found"
    fi
    
    # Check if DocumentDB exists
    if kubectl --context "$cluster" get documentdb documentdb-preview -n documentdb-preview-ns &>/dev/null; then
      echo "✓ DocumentDB resource exists"
      
      # Get DocumentDB status
      STATUS=$(kubectl --context "$cluster" get documentdb documentdb-preview -n documentdb-preview-ns -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
      echo "  Status: $STATUS"
      
      # Check if this is the primary or replica
      PRIMARY=$(kubectl --context "$cluster" get documentdb documentdb-preview -n documentdb-preview-ns -o jsonpath='{.spec.clusterReplication.primary}' 2>/dev/null || echo "")
      if [ "$PRIMARY" = "$cluster" ]; then
        echo "  Role: PRIMARY"
      else
        echo "  Role: REPLICA (Primary: $PRIMARY)"
      fi
    else
      echo "✗ DocumentDB resource not found"
    fi
    
    # Check pods
    PODS=$(kubectl --context "$cluster" get pods -n documentdb-preview-ns --no-headers 2>/dev/null | wc -l || echo "0")
    echo "  Pods: $PODS"
    
    # Show pod status if any exist
    if [ "$PODS" -gt 0 ]; then
      kubectl --context "$cluster" get pods -n documentdb-preview-ns -o wide 2>/dev/null | head -5
    fi
  else
    echo "✗ Namespace not found (resources may still be propagating)"
  fi
done

echo ""
echo "======================================="
echo "Connection Information"
echo "======================================="
echo ""
echo "Username: default_user"
echo "Password: $DOCUMENTDB_PASSWORD"
echo ""
echo "To connect to the primary cluster (eastus2):"
# TBD
echo ""
echo "Connection string:"
# Add conenction string from Rayhan's stuff
echo ""
echo "To initiate failover to a different region (e.g., westus3):"
echo "kubectl --context $HUB_CONTEXT patch documentdb documentdb-preview -n documentdb-preview-ns \\"
echo "  --type='json' -p='["
echo "  {\"op\": \"replace\", \"path\": \"/spec/clusterReplication/primary\", \"value\":\"member-westus3-z2fyhq65f4ktg\"},"
echo "  {\"op\": \"replace\", \"path\": \"/spec/clusterReplication/clusterList\", \"value\":[\"member-westus3-z2fyhq65f4ktg\", \"member-eastus2-z2fyhq65f4ktg\", \"member-uksouth-z2fyhq65f4ktg\"]}"
echo "  ]'"
echo ""
echo "To monitor the deployment:"
echo "watch 'kubectl --context $HUB_CONTEXT get clusterresourceplacement documentdb-crp -o wide'"