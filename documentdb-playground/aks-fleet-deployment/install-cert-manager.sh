#!/bin/bash
# Install cert-manager on all member clusters in the fleet

set -euo pipefail

RESOURCE_GROUP="${RESOURCE_GROUP:-documentdb-aks-fleet-rg}"
HUB_REGION="${HUB_REGION:-westus3}"
MEMBERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name')
echo -e "Members:\n$MEMBERS"

# Ensure contexts and get hub name
for cluster in $MEMBERS; do
  echo "Fetching creds for $cluster..."
  az aks get-credentials -g "$RESOURCE_GROUP" -n "$cluster" --overwrite-existing
  if [[ "$cluster" == *"$HUB_REGION"* ]]; then HUB_CLUSTER="$cluster"; fi
done

helm repo add jetstack https://charts.jetstack.io 
helm repo update 

# Install cert manager on hub cluster
echo -e "\nInstalling cert-manager on $HUB_CLUSTER..."
kubectl config use-context "$HUB_CLUSTER" 
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true 1>/dev/null || true
kubectl rollout status deployment/cert-manager -n cert-manager --timeout=240s || true
echo "Pods ($cluster):"
kubectl get pods -n cert-manager -o wide || true

# Verify we can talk to the hub API
echo "Verifying API connectivity to hub context ($HUB_CONTEXT)..."
if ! kubectl --context "$HUB_CONTEXT" get ns ; then
  echo "Error: unable to talk to cluster using context '$HUB_CONTEXT'. Check credentials and RBAC." >&2
  kubectl --context "$HUB_CONTEXT" config view --minify
  exit 1
fi

# Install cert-manager CRDs on the hub context (safe to re-apply)
echo "Applying cert-manager CRDs on hub ($HUB_CONTEXT)..."
kubectl --context "$HUB_CONTEXT" apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.crds.yaml

echo -e "\nDone."
