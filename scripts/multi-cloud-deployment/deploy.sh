#!/usr/bin/env bash
# Consolidated AKS Fleet and DocumentDB Deployment Script
# This script combines all deployment steps into a single workflow

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================

RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"
RG_LOCATION="${RG_LOCATION:-eastus2}"
HUB_REGION="${HUB_REGION:-$RG_LOCATION}"
TEMPLATE_DIR="$(dirname "$0")"
HUB_VM_SIZE="${HUB_VM_SIZE:-}"
VERSION="${VERSION:-200}"
VALUES_FILE="${VALUES_FILE:-}"
ISTIO_DIR="${ISTIO_DIR:-}"
AKS_REGION="${AKS_REGION:-eastus2}"
HUB_CONTEXT="${HUB_CONTEXT:-hub}"

PROJECT_ID="${PROJECT_ID:-sanguine-office-475117-s6}"
GCP_USER="${GCP_USER:-alexanderlaye59@gmail.com}"
ZONE="${ZONE:-us-central1-a}"
GKE_CLUSTER_NAME="${GKE_CLUSTER_NAME:-gke-documentdb-cluster}"

# ============================================================================
# Helper Functions
# ============================================================================

run() { echo "+ $*"; "$@"; }

check_prerequisites() {
  echo "Checking prerequisites..."

  # Check Azure CLI
  if ! command -v az &> /dev/null; then
    echo "ERROR: Azure CLI not found. Please install Azure CLI first." >&2
    exit 1
  fi

  # Check kubectl
  if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl not found. Please install kubectl first." >&2
    exit 1
  fi

  # Check Helm
  if ! command -v helm &> /dev/null; then
    echo "ERROR: Helm not found. Please install Helm first." >&2
    exit 1
  fi

  # Check gcloud CLI
  if ! command -v gcloud &> /dev/null; then
    echo "ERROR: gcloud CLI not found. Please install Google Cloud SDK first." >&2
    exit 1
  fi

  # Check Azure login
  if ! az account show &> /dev/null; then
    echo "ERROR: Not logged into Azure. Please run 'az login' first." >&2
    exit 1
  fi

  echo "✅ All prerequisites met"
}

wait_for_no_inprogress() {
  local rg="$1"
  echo "Checking for in-progress AKS operations in resource group '$rg'..."
  local inprogress
  inprogress=$(az aks list -g "$rg" -o json \
    | jq -r '.[] | select(.provisioningState != "Succeeded" and .provisioningState != null) | [.name, .provisioningState] | @tsv')

  if [ -z "$inprogress" ]; then
    echo "No in-progress AKS operations detected."
    return 0
  fi

  echo "Found clusters still provisioning:" 
  echo "$inprogress" | while IFS=$'\t' read -r name state; do echo "  - $name: $state"; done
  echo "Please re-run this script after the above operations complete." >&2
  return 1
}

# ============================================================================
# Step 1: Deploy AKS Fleet Infrastructure
# ============================================================================

check_prerequisites

echo "Creating or using resource group..."
EXISTING_RG_LOCATION=$(az group show --name "$RESOURCE_GROUP" --query location -o tsv 2>/dev/null || true)
if [ -n "$EXISTING_RG_LOCATION" ]; then
  echo "Using existing resource group '$RESOURCE_GROUP' in location '$EXISTING_RG_LOCATION'"
  RG_LOCATION="$EXISTING_RG_LOCATION"
else
  az group create --name "$RESOURCE_GROUP" --location "$RG_LOCATION"
fi

echo "Deploying AKS Fleet with Bicep..."
if ! wait_for_no_inprogress "$RESOURCE_GROUP"; then
  echo "Exiting without changes due to in-progress operations." >&2
  exit 1
fi

PARAMS=(
  --parameters "$TEMPLATE_DIR/parameters.bicepparam"
  --parameters hubRegion="$HUB_REGION"
  --parameters memberRegion="$AKS_REGION"
)

if [ -n "$HUB_VM_SIZE" ]; then
  echo "Overriding hubVmSize with: $HUB_VM_SIZE"
  PARAMS+=( --parameters hubVmSize="$HUB_VM_SIZE" )
fi

DEPLOYMENT_NAME="aks-fleet-$(date +%s)"
az deployment group create \
  --name "$DEPLOYMENT_NAME" \
  --resource-group $RESOURCE_GROUP \
  --template-file "$TEMPLATE_DIR/main.bicep" \
  "${PARAMS[@]}" >/dev/null

# Retrieve outputs
DEPLOYMENT_OUTPUT=$(az deployment group show \
  --resource-group $RESOURCE_GROUP \
  --name "$DEPLOYMENT_NAME" \
  --query "properties.outputs" -o json)

FLEET_NAME=$(echo $DEPLOYMENT_OUTPUT | jq -r '.fleetName.value')
FLEET_ID_FROM_OUTPUT=$(echo $DEPLOYMENT_OUTPUT | jq -r '.fleetId.value')
AKS_CLUSTER_NAME=$(echo $DEPLOYMENT_OUTPUT | jq -r '.memberClusterName.value')

SUBSCRIPTION_ID=$(az account show --query id -o tsv)
export FLEET_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ContainerService/fleets/${FLEET_NAME}"

# Set up RBAC
echo "Setting up RBAC access for Fleet..."
export IDENTITY=$(az ad signed-in-user show --query "id" --output tsv)
export ROLE="Azure Kubernetes Fleet Manager RBAC Cluster Admin"
echo "Assigning role '$ROLE' to user '$IDENTITY'..."
az role assignment create --role "${ROLE}" --assignee ${IDENTITY} --scope ${FLEET_ID} >/dev/null 2>&1 || true

# ============================================================================
# Step 1.2: Deploy GKE Infrastructure
# ============================================================================

gcloud config set account $GCP_USER
gcloud auth login --brief
# TODO move this to a check at the top
# sudo apt-get install google-cloud-cli-gke-gcloud-auth-plugin

# Create project if it doesn't exist
if ! gcloud projects describe $PROJECT_ID &>/dev/null; then
  gcloud projects create $PROJECT_ID
fi

gcloud config set project $PROJECT_ID

gcloud services enable container.googleapis.com
gcloud projects add-iam-policy-binding $PROJECT_ID --member="user:$GCP_USER" --role="roles/container.admin"
gcloud projects add-iam-policy-binding $PROJECT_ID --member="user:$GCP_USER" --role="roles/compute.networkAdmin"
gcloud projects add-iam-policy-binding $PROJECT_ID --member="user:$GCP_USER" --role="roles/iam.serviceAccountUser"

# Delete cluster if it exists
if gcloud container clusters describe "$GKE_CLUSTER_NAME" --zone "$ZONE" --project $PROJECT_ID &>/dev/null; then
  gcloud container clusters delete "$GKE_CLUSTER_NAME" \
    --zone "$ZONE" \
    --project $PROJECT_ID  \
    --quiet
fi

gcloud container clusters create "$GKE_CLUSTER_NAME" \
    --zone "$ZONE" \
    --num-nodes "2" \
    --machine-type "e2-standard-4" \
    --enable-ip-access \
    --project $PROJECT_ID

# ============================================================================
# Step 1.3: Collect connection details
# ============================================================================

MEMBER_CLUSTER_NAMES=("$AKS_CLUSTER_NAME" "$GKE_CLUSTER_NAME")

# Fetch kubeconfig contexts
echo "Fetching kubeconfig contexts..."
az fleet get-credentials --resource-group "$RESOURCE_GROUP" --name "$FLEET_NAME" --overwrite-existing

az aks get-credentials --resource-group "$RESOURCE_GROUP" --name "$AKS_CLUSTER_NAME" --overwrite-existing 


kubectl config delete-context "$GKE_CLUSTER_NAME" || true
kubectl config delete-cluster "$GKE_CLUSTER_NAME" || true
kubectl config delete-user "$GKE_CLUSTER_NAME" || true
gcloud container clusters get-credentials "$GKE_CLUSTER_NAME" \
    --location="$ZONE"
fullName=$(kubectl config current-context)
# Replace all occurrences of the generated name with GKE_CLUSTER_NAME in kubeconfig
sed -i "s|$fullName|$GKE_CLUSTER_NAME|g" ~/.kube/config

echo "✅ Fleet infrastructure deployed successfully"
echo "Fleet Name: $FLEET_NAME"
echo "Fleet ID: $FLEET_ID"
echo "Member Clusters:"
echo "$AKS_CLUSTER_NAME"
echo "$GKE_CLUSTER_NAME"

# ============================================================================
# Step 1.4: Join member clusters to fleet
# ============================================================================

temp_dir=$(mktemp -d)
echo "Temporary directory created at: $temp_dir"
pushd $temp_dir
git clone https://github.com/kubefleet-dev/kubefleet.git
git clone https://github.com/Azure/fleet-networking.git
pushd $temp_dir/kubefleet
chmod +x hack/membership/joinMC.sh
hack/membership/joinMC.sh "v0.16.5" "$HUB_CONTEXT" "$GKE_CLUSTER_NAME"
popd

# TODO clean this up a bit
echo "Waiting for $GKE_CLUSTER_NAME to join fleet..."
kubectl --context $HUB_CONTEXT wait --for=jsonpath='{.status.resourceUsage.observationTime}' membercluster/$GKE_CLUSTER_NAME 

pushd $temp_dir/fleet-networking
chmod +x hack/membership/joinMC.sh 
hack/membership/joinMC.sh "v0.16.5" "v0.3.24" $HUB_CONTEXT $GKE_CLUSTER_NAME
popd

# TODO fix this
# kubectl --context $HUB_CONTEXT wait --for=jsonpath='{.status.agentStatus[?(@.conditions[?(@.reason=="AgentJoined" && @.status=="True")])].type}' membercluster/$GKE_CLUSTER_NAME

popd

# ============================================================================
# Step 2: Install cert-manager on all member clusters
# ============================================================================

helm repo add jetstack https://charts.jetstack.io 2>/dev/null || true
helm repo update >/dev/null 2>&1

for cluster in ${MEMBER_CLUSTER_NAMES[@]}; do
  echo "Installing cert-manager on $cluster..."
  kubectl config use-context "$cluster" 2>/dev/null
  helm upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set installCRDs=true \
    --wait --timeout=5m >/dev/null 2>&1 || echo "  Warning: cert-manager installation issue on $cluster"
  echo "✓ cert-manager installed on $cluster"
done

echo "✅ cert-manager installed on all clusters"

# ============================================================================
# Step 3: Install Istio and setup mesh
# ============================================================================

# Create an issuer in istio-system namespace on hub
temp_dir=$(mktemp -d)
echo "Temporary directory created at: $temp_dir"

# Check if istioctl is installed, if not install it to temp_dir
if ! command -v istioctl &> /dev/null; then
  echo "istioctl not found, installing to $temp_dir..."
  ISTIO_VERSION="1.24.0"
  curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$ISTIO_VERSION TARGET_ARCH=x86_64 sh - -d "$temp_dir" >/dev/null 2>&1
  export PATH="$temp_dir/istio-$ISTIO_VERSION/bin:$PATH"
  echo "✓ istioctl installed to $temp_dir/istio-$ISTIO_VERSION/bin"
else
  echo "✓ istioctl already installed: $(which istioctl)"
fi

if [ -z "$ISTIO_DIR" ]; then
  git clone https://github.com/istio/istio.git "$temp_dir/istio"
  export ISTIO_DIR="$temp_dir/istio"
fi
rm -rf "$TEMPLATE_DIR/certs"
mkdir $TEMPLATE_DIR/certs
pushd $TEMPLATE_DIR/certs
make -f "$ISTIO_DIR/tools/certs/Makefile.selfsigned.mk" root-ca
index=1
for cluster in ${MEMBER_CLUSTER_NAMES[@]}; do
  make -f "$ISTIO_DIR/tools/certs/Makefile.selfsigned.mk" "${cluster}-cacerts"
  kubectl --context "$cluster" delete namespace/istio-system --wait=true --ignore-not-found=true
  kubectl --context "$cluster" create namespace istio-system
  kubectl --context "$cluster" wait --for=jsonpath='{.status.phase}'=Active namespace/istio-system --timeout=60s
  # create certs
  kubectl --context "$cluster" create secret generic cacerts -n istio-system \
        --from-file="${cluster}/ca-cert.pem" \
        --from-file="${cluster}/ca-key.pem" \
        --from-file="${cluster}/root-cert.pem" \
        --from-file="${cluster}/cert-chain.pem"

  kubectl --context="${cluster}" label namespace istio-system topology.istio.io/network=network${index}

  #install istio on each cluster
  cat <<EOF | istioctl --context "$cluster" apply -y -f -
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: ${cluster}
      network: network${index}
EOF

  $ISTIO_DIR/samples/multicluster/gen-eastwest-gateway.sh \
    --network network${index} | \
    istioctl --context="${cluster}" install -y -f -

  kubectl --context="${cluster}" apply -n istio-system -f \
    $ISTIO_DIR/samples/multicluster/expose-services.yaml

  index=$((index + 1))
done

# only after everything else is done, make the secrets
for cluster in ${MEMBER_CLUSTER_NAMES[@]}; do
  remoteSecretFile=$temp_dir/${cluster}-remote-secret.yaml
  istioctl create-remote-secret \
      --context="${cluster}" \
      --name="${cluster}" > $remoteSecretFile
  for other_cluster in ${MEMBER_CLUSTER_NAMES[@]}; do
    if [ "$cluster" = "$other_cluster" ]; then
      continue
    fi
      kubectl apply -f $remoteSecretFile --context="${other_cluster}"
  done
done

popd

# ============================================================================
# Step 4: Install DocumentDB Operator
# ============================================================================

CHART_DIR="$(cd "$TEMPLATE_DIR/../.." && pwd)/documentdb-chart"
CHART_PKG="$TEMPLATE_DIR/documentdb-operator-0.0.${VERSION}.tgz"

# Apply cert-manager CRDs on hub
echo "Applying cert-manager CRDs on hub ($HUB_CONTEXT)..."
kubectl --context "$HUB_CONTEXT" apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.crds.yaml #>/dev/null 2>&1

# Create documentdb-operator namespace with Istio injection on hub
cat <<EOF | kubectl --context "$HUB_CONTEXT" apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: documentdb-operator
  labels:
    istio-injection: enabled
EOF

# Build/package chart if not present
if [ ! -f "$CHART_PKG" ] && [ -d "$CHART_DIR" ]; then
  echo "Packaging chart..."
  helm dependency update "$CHART_DIR" >/dev/null 2>&1
  helm package "$CHART_DIR" --version 0.0."${VERSION}" --destination "$TEMPLATE_DIR" >/dev/null 2>&1
fi

# Install operator
echo "Installing DocumentDB operator on hub ($HUB_CONTEXT)..."
if [ -f "$CHART_PKG" ]; then
  if [ -n "$VALUES_FILE" ] && [ -f "$VALUES_FILE" ]; then
    helm upgrade --install documentdb-operator "$CHART_PKG" \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT" \
      --values "$VALUES_FILE"
  else
    helm upgrade --install documentdb-operator "$CHART_PKG" \
      --namespace documentdb-operator \
      --kube-context "$HUB_CONTEXT"
  fi
fi

kubectl --context "$HUB_CONTEXT" apply -f "$TEMPLATE_DIR/documentdb-base.yaml" 

echo "✅ DocumentDB operator installed"

# Verify operator on member clusters
echo "Verifying operator deployment..."
for cluster in ${MEMBER_CLUSTER_NAMES[@]}; do
  READY=$(kubectl --context "$cluster" get deploy documentdb-operator -n documentdb-operator -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
  DESIRED=$(kubectl --context "$cluster" get deploy documentdb-operator -n documentdb-operator -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
  echo "  $cluster: $READY/$DESIRED replicas ready"
done

# ============================================================================
# Save environment variables and aliases
# ============================================================================
