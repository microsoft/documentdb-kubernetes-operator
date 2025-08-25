#!/usr/bin/env bash
# filepath: /Users/geeichbe/Projects/documentdb-kubernetes-operator/scripts/aks-fleet-deployment/deploy-multi-region.sh
set -euo pipefail

# Deploy multi-region DocumentDB using Fleet
# Usage: ./deploy-multi-region.sh [password]

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Resource group
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

# Load environment if available
if [ -f "$HOME/.bashrc" ]; then
  source "$HOME/.bashrc" || true
fi

# Dynamically get member clusters from Azure
echo "Discovering member clusters in resource group: $RESOURCE_GROUP..."
MEMBER_CLUSTERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name' | sort)

if [ -z "$MEMBER_CLUSTERS" ]; then
  echo "Error: No member clusters found in resource group $RESOURCE_GROUP"
  echo "Please ensure the fleet is deployed first using ./deploy-fleet-bicep.sh"
  exit 1
fi

# Convert to array
CLUSTER_ARRAY=($MEMBER_CLUSTERS)
echo "Found ${#CLUSTER_ARRAY[@]} member clusters:"
for cluster in "${CLUSTER_ARRAY[@]}"; do
  echo "  - $cluster"
done

# Select primary cluster (prefer eastus2, or use first cluster)
PRIMARY_CLUSTER=""
for cluster in "${CLUSTER_ARRAY[@]}"; do
  if [[ "$cluster" == *"eastus2"* ]]; then
    PRIMARY_CLUSTER="$cluster"
    break
  fi
done

# If no eastus2 cluster found, use the first one
if [ -z "$PRIMARY_CLUSTER" ]; then
  PRIMARY_CLUSTER="${CLUSTER_ARRAY[0]}"
fi

echo ""
echo "Selected primary cluster: $PRIMARY_CLUSTER"

# Build the cluster list YAML with proper indentation
CLUSTER_LIST=""
for cluster in "${CLUSTER_ARRAY[@]}"; do
  if [ -z "$CLUSTER_LIST" ]; then
    CLUSTER_LIST="      - ${cluster}"
  else
    CLUSTER_LIST="${CLUSTER_LIST}"$'\n'"      - ${cluster}"
  fi
done

# Step 1: Create cluster identification ConfigMaps on each member cluster
echo ""
echo "======================================="
echo "Creating cluster identification ConfigMaps..."
echo "======================================="

for cluster in "${CLUSTER_ARRAY[@]}"; do
  echo ""
  echo "Processing ConfigMap for $cluster..."
  
  # Check if context exists
  if ! kubectl config get-contexts "$cluster" &>/dev/null; then
    echo "✗ Context $cluster not found, skipping"
    continue
  fi
  
  # Extract region from cluster name (member-<region>-<suffix>)
  REGION=$(echo "$cluster" | awk -F- '{print $2}')
  
  # Create or update the cluster-name ConfigMap
  kubectl --context "$cluster" create configmap cluster-name \
    -n kube-system \
    --from-literal=name="$cluster" \
    --from-literal=region="$REGION" \
    --dry-run=client -o yaml | kubectl --context "$cluster" apply -f -
  
  # Verify the ConfigMap was created
  if kubectl --context "$cluster" get configmap cluster-name -n kube-system &>/dev/null; then
    echo "✓ ConfigMap created/updated for $cluster (region: $REGION)"
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
  HUB_CONTEXT="${CLUSTER_ARRAY[0]}"
  if [ -z "$HUB_CONTEXT" ]; then
    echo "Error: No suitable context found. Please ensure you have credentials for the fleet."
    exit 1
  fi
fi

echo "Using hub context: $HUB_CONTEXT"

# Check if resources already exist
EXISTING_RESOURCES=""
if kubectl --context "$HUB_CONTEXT" get namespace documentdb-preview-ns &>/dev/null 2>&1; then
  EXISTING_RESOURCES="${EXISTING_RESOURCES}namespace "
fi
if kubectl --context "$HUB_CONTEXT" get secret documentdb-credentials -n documentdb-preview-ns &>/dev/null 2>&1; then
  EXISTING_RESOURCES="${EXISTING_RESOURCES}secret "
fi
if kubectl --context "$HUB_CONTEXT" get documentdb documentdb-preview -n documentdb-preview-ns &>/dev/null 2>&1; then
  EXISTING_RESOURCES="${EXISTING_RESOURCES}documentdb "
fi
if kubectl --context "$HUB_CONTEXT" get clusterresourceplacement documentdb-crp &>/dev/null 2>&1; then
  EXISTING_RESOURCES="${EXISTING_RESOURCES}clusterresourceplacement "
fi

if [ -n "$EXISTING_RESOURCES" ]; then
  echo ""
  echo "⚠️  Warning: The following resources already exist: $EXISTING_RESOURCES"
  echo ""
  echo "Options:"
  echo "1. Delete existing resources and redeploy (data will be lost)"
  echo "2. Update existing deployment (preserve data)"
  echo "3. Cancel"
  echo ""
  read -p "Choose an option (1/2/3): " CHOICE
  
  case $CHOICE in
    1)
      echo "Deleting existing resources..."
      kubectl --context "$HUB_CONTEXT" delete clusterresourceplacement documentdb-crp --ignore-not-found=true
      kubectl --context "$HUB_CONTEXT" delete namespace documentdb-preview-ns --ignore-not-found=true
      echo "Waiting for namespace deletion to complete..."
      kubectl --context "$HUB_CONTEXT" wait --for=delete namespace/documentdb-preview-ns --timeout=60s 2>/dev/null || true
      ;;
    2)
      echo "Updating existing deployment..."
      ;;
    3)
      echo "Cancelled."
      exit 0
      ;;
    *)
      echo "Invalid choice. Cancelled."
      exit 1
      ;;
  esac
fi

# Create a temporary file with substituted values
TEMP_YAML=$(mktemp)

# Use sed for safer substitution
sed -e "s/\${DOCUMENTDB_PASSWORD:-DefaultP@ssw0rd123}/$DOCUMENTDB_PASSWORD/g" \
    -e "s/\${PRIMARY_CLUSTER}/$PRIMARY_CLUSTER/g" \
    "$SCRIPT_DIR/multi-region.yaml" | \
while IFS= read -r line; do
  if [[ "$line" == '${CLUSTER_LIST}' ]]; then
    echo "$CLUSTER_LIST"
  else
    echo "$line"
  fi
done > "$TEMP_YAML"

# Debug: show the generated YAML section with clusterReplication
echo ""
echo "Generated configuration preview:"
echo "--------------------------------"
echo "Primary cluster: $PRIMARY_CLUSTER"
echo "Cluster list:"
echo "$CLUSTER_LIST"
echo "--------------------------------"

# cat "$TEMP_YAML" 

# Apply the configuration
echo ""
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

for cluster in "${CLUSTER_ARRAY[@]}"; do
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
    REGION=$(kubectl --context "$cluster" get configmap cluster-name -n kube-system -o jsonpath='{.data.region}')
    echo "✓ Cluster identified as: $CLUSTER_ID (region: $REGION)"
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
      if [ "$cluster" = "$PRIMARY_CLUSTER" ]; then
        echo "  Role: PRIMARY"
      else
        echo "  Role: REPLICA"
      fi
    else
      echo "✗ DocumentDB resource not found"
    fi
    
    # Check pods
    PODS=$(kubectl --context "$cluster" get pods -n documentdb-preview-ns --no-headers 2>/dev/null | wc -l || echo "0")
    echo "  Pods: $PODS"
    
    # Show pod status if any exist
    if [ "$PODS" -gt 0 ]; then
      kubectl --context "$cluster" get pods -n documentdb-preview-ns 2>/dev/null | head -5
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
echo "To connect to the primary cluster ($PRIMARY_CLUSTER):"
echo "kubectl --context $PRIMARY_CLUSTER port-forward -n documentdb-preview-ns svc/documentdb-preview 10260:10260"
echo ""
echo "Connection string:"
kubectl --context $PRIMARY_CLUSTER get documentdb -n documentdb-preview-ns  -A -o json | jq ".items[0].status.connectionString"
echo ""

# Generate failover commands for all non-primary clusters
echo "To initiate failover to a different region:"
for cluster in "${CLUSTER_ARRAY[@]}"; do
  if [ "$cluster" != "$PRIMARY_CLUSTER" ]; then
    REGION=$(echo "$cluster" | awk -F- '{print $2}')
    echo ""
    echo "# Failover to $REGION:"
    echo "kubectl --context $HUB_CONTEXT patch documentdb documentdb-preview -n documentdb-preview-ns \\"
    echo "  --type='merge' -p '{\"spec\":{\"clusterReplication\":{\"primary\":\"$cluster\"}}}'"
  fi
done

echo ""
echo "To monitor the deployment:"
echo "watch 'kubectl --context $HUB_CONTEXT get clusterresourceplacement documentdb-crp -o wide'"

echo ""
echo "To check DocumentDB status across all clusters:"
# Create a space-separated string from the array
CLUSTER_STRING=$(IFS=' '; echo "${CLUSTER_ARRAY[*]}")
echo "for c in $CLUSTER_STRING; do echo \"=== \$c ===\"; kubectl --context \$c get documentdb,pods -n documentdb-preview-ns 2>/dev/null || echo 'Not deployed yet'; echo; done"