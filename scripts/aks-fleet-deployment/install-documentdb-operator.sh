#!/usr/bin/env bash
set -euo pipefail

# Install DocumentDB operator on a hub (or a chosen member) cluster.
# Strategy:
#  - Prefer hub context if available and functional.
#  - Otherwise, pick the first member cluster in the fleet RG and fetch admin credentials.
#  - Ensure cert-manager CRDs are installed on the hub context.
#  - Package the local chart (if needed) and install the operator via Helm.

RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"
HUB_CONTEXT=${HUB_CONTEXT:-hub}
CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/documentdb-chart"
VERSION="${VERSION:-200}"
VALUES_FILE="${VALUES_FILE:-}"

# Helper: print and run
run() { echo "+ $*"; "$@"; }

# Make sure we have the basics
for cmd in az kubectl helm jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: required command '$cmd' not found. Install it and re-run." >&2
    exit 1
  fi
done

# Verify we can talk to the hub API
echo "Verifying API connectivity to hub context ($HUB_CONTEXT)..."
if ! kubectl --context "$HUB_CONTEXT" get ns ; then
  echo "Error: unable to talk to cluster using context '$HUB_CONTEXT'. Check credentials and RBAC." >&2
  kubectl --context "$HUB_CONTEXT" config view --minify
  exit 1
fi

# Install cert-manager CRDs on the hub context (safe to re-apply)
echo "Applying cert-manager CRDs on hub ($HUB_CONTEXT)..."
run kubectl --context "$HUB_CONTEXT" apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.crds.yaml

# Build/package chart if local tgz not present
CHART_PKG="./documentdb-operator-0.0.${VERSION}.tgz"
if [ ! -f "$CHART_PKG" ]; then
  echo "Packaging chart (helm dependency update && helm package)..."
  run helm dependency update "$CHART_DIR"
  run helm package "$CHART_DIR" --version 0.0."${VERSION}"
fi

# Install/upgrade operator using the packaged chart if available, otherwise fallback to OCI registry
if [ -f "$CHART_PKG" ]; then
  echo "Installing operator from package $CHART_PKG into namespace documentdb-operator on context $HUB_CONTEXT"
  if [ -n "$VALUES_FILE" ] && [ -f "$VALUES_FILE" ]; then
    echo "Using values file: $VALUES_FILE"
    run helm upgrade --install documentdb-operator "$CHART_PKG" \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT" \
      --create-namespace \
      --values "$VALUES_FILE"
  else
    run helm upgrade --install documentdb-operator "$CHART_PKG" \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT" \
      --create-namespace
  fi
else
  echo "Package not found. Installing from OCI registry (requires helm v3.8+)..."
  if [ -n "$VALUES_FILE" ] && [ -f "$VALUES_FILE" ]; then
    echo "Using values file: $VALUES_FILE"
    run helm upgrade --install documentdb-operator \
      oci://ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-operator \
      --version 0.0.1 \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT" \
      --create-namespace \
      --values "$VALUES_FILE"
  else
    run helm upgrade --install documentdb-operator \
      oci://ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-operator \
      --version 0.0.1 \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT" \
      --create-namespace
  fi
fi

kubectl --context "$HUB_CONTEXT"  apply -f ./documentdb-base.yaml

# Get all member clusters
MEMBER_CLUSTERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name')

# Show status on all member clusters
echo "Checking operator status on all member clusters..."

for CLUSTER in $MEMBER_CLUSTERS; do
  echo ""
  echo "======================================="
  echo "Cluster: $CLUSTER"
  echo "======================================="
  
  # Get the context name for this cluster
  CONTEXT="$CLUSTER"

  echo "Checking rollout status on $CLUSTER..."
  set +e
  kubectl --context "$CONTEXT" -n documentdb-operator rollout status deployment/documentdb-operator --timeout=300s
  rc=$?
  set -e
  if [ $rc -ne 0 ]; then
    echo "Warning: operator rollout didn't complete within timeout on $CLUSTER. Check pods:"
    kubectl --context "$CLUSTER" get pods -n documentdb-operator 2>/dev/null || echo "  Unable to get pods on $CLUSTER"
  else
    echo "✓ Operator rollout completed successfully on $CLUSTER"
  fi
  echo ""
  
  echo "Operator deployments in $CLUSTER:"
  kubectl --context "$CONTEXT" get deploy -n documentdb-operator -o wide 2>/dev/null || echo "  No deployments found or unable to connect"
  
  echo ""
  echo "Operator pods in $CLUSTER:"
  kubectl --context "$CONTEXT" get pods -n documentdb-operator -o wide 2>/dev/null || echo "  No pods found or unable to connect"
  
  echo ""
  echo "cnpg-system pods in $CLUSTER:"
  kubectl --context "$CONTEXT" get pods -n cnpg-system -o wide 2>/dev/null || echo "  No pods found or unable to connect"
done

echo ""
echo "======================================="
echo "Summary of operator status across all member clusters"
echo "======================================="

# Show a summary table
echo ""
echo "Deployment Status Summary:"
for CLUSTER in $MEMBER_CLUSTERS; do
  READY=$(kubectl --context "$CLUSTER" get deploy documentdb-operator -n documentdb-operator -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
  DESIRED=$(kubectl --context "$CLUSTER" get deploy documentdb-operator -n documentdb-operator -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
  echo "  $CLUSTER: $READY/$DESIRED replicas ready"
done

echo ""
echo "Done. If any commands failed due to permissions, ensure your Azure account has contributor/AKS admin permissions in resource group '$RESOURCE_GROUP'."
