#!/bin/bash
# Install cert-manager on all member clusters in the fleet

set -euo pipefail

RESOURCE_GROUP="${RESOURCE_GROUP:-documentdb-aks-fleet-rg}"
MEMBERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name')
echo -e "Members:\n$MEMBERS"

# Ensure contexts
for C in $MEMBERS; do
  echo "Fetching creds for $C..."
  az aks get-credentials -g "$RESOURCE_GROUP" -n "$C" --overwrite-existing 1>/dev/null || true
done

# Helm repo and install per member
helm repo add jetstack https://charts.jetstack.io 1>/dev/null || true
helm repo update 1>/dev/null || true

for C in $MEMBERS; do
  echo -e "\nInstalling cert-manager on $C..."
  kubectl config use-context "$C" 1>/dev/null
  helm upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set installCRDs=true 1>/dev/null || true
  kubectl rollout status deployment/cert-manager -n cert-manager --timeout=240s || true
  echo "Pods ($C):"
  kubectl get pods -n cert-manager -o wide || true
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

echo -e "\nDone."
