#!/usr/bin/env bash
# filepath: /Users/geeichbe/Projects/documentdb-kubernetes-operator/scripts/aks-fleet-deployment/deploy-multi-region.sh
set -euo pipefail

# Deploy multi-region DocumentDB using Fleet
# Usage: ./deploy-multi-region.sh [password]

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Set password from argument or environment variable
DOCUMENTDB_PASSWORD="${1:-${DOCUMENTDB_PASSWORD:-}}"

RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"

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

# Check deployment status on each member cluster
echo ""
echo "Checking deployment status on member clusters..."

# Get all member clusters
MEMBER_CLUSTERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name')


for cluster in $MEMBER_CLUSTERS; do
  echo ""
  echo "=== $cluster ==="
  
  # Check if context exists
  if ! kubectl config get-contexts "$cluster" &>/dev/null; then
    echo "✗ Context not found, skipping"
    continue
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
    else
      echo "✗ DocumentDB resource not found"
    fi
    
    # Check pods
    PODS=$(kubectl --context "$cluster" get pods -n documentdb-preview-ns --no-headers 2>/dev/null | wc -l || echo "0")
    echo "  Pods: $PODS"
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
echo "kubectl --context member-eastus2-z2fyhq65f4ktg port-forward -n documentdb-preview-ns svc/documentdb-preview 5432:5432"
echo ""
echo "Connection string:"
# TODO: Needs to be the kubectl get status Rayhan added
echo ""
echo "To monitor the deployment:"
echo "watch 'kubectl --context $HUB_CONTEXT get clusterresourceplacement documentdb-crp -o wide'"