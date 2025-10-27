#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: gateway-tls-e2e.sh [options]

Automates the end-to-end walkthrough from docs/gateway-tls-validation.md:
  1. Provision AKS prerequisites
  2. Validate the self-signed TLS flow
  3. Prepare Azure Key Vault assets
  4. Transition to provided TLS and validate connectivity

Options:
      --suffix <value>         String used to derive resource names (default: current timestamp)
      --location <region>      Azure region for the resources (default: eastus2)
      --resource-group <name>  Azure resource group for AKS/Key Vault (default: guanzhou-<suffix>-rg)
      --aks-name <name>        AKS cluster name (default: guanzhou-<suffix>)
      --keyvault <name>        Azure Key Vault name (default: ddb-issuer-<suffix>)
      --namespace <name>       Kubernetes namespace for DocumentDB (default: documentdb-preview-ns)
      --docdb-name <name>      DocumentDB resource name (default: documentdb-preview)
  --github-username <val>  GitHub username for operator install (optional)
  --github-token <val>     GitHub token with read:packages scope (optional)
      --skip-cluster           Assume AKS cluster already exists and skip creation
      --help                  Show this message
EOF
}

SUFFIX="$(date +%m%d%H%M)"
LOCATION="eastus2"
RESOURCE_GROUP=""
AKS_NAME=""
KEYVAULT_NAME=""
NAMESPACE="documentdb-preview-ns"
DOCDB_NAME="documentdb-preview"
SKIP_CLUSTER=0
GITHUB_USERNAME=""
GITHUB_TOKEN=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --suffix)
      SUFFIX="$2"; shift 2 ;;
    --location)
      LOCATION="$2"; shift 2 ;;
    --resource-group)
      RESOURCE_GROUP="$2"; shift 2 ;;
    --aks-name)
      AKS_NAME="$2"; shift 2 ;;
    --keyvault)
      KEYVAULT_NAME="$2"; shift 2 ;;
    --namespace)
      NAMESPACE="$2"; shift 2 ;;
    --docdb-name)
      DOCDB_NAME="$2"; shift 2 ;;
    --github-username)
      GITHUB_USERNAME="$2"; shift 2 ;;
    --github-token)
      GITHUB_TOKEN="$2"; shift 2 ;;
    --skip-cluster)
      SKIP_CLUSTER=1; shift ;;
    --help|-h)
      usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1 ;;
  esac
done

if [[ -z "$RESOURCE_GROUP" ]]; then
  RESOURCE_GROUP="guanzhou-${SUFFIX}-rg"
fi
if [[ -z "$AKS_NAME" ]]; then
  AKS_NAME="guanzhou-${SUFFIX}"
fi
if [[ -z "$KEYVAULT_NAME" ]]; then
  KEYVAULT_NAME="ddb-issuer-${SUFFIX}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

create_cluster_script="$SCRIPT_DIR/create-cluster.sh"
setup_selfsigned_script="$SCRIPT_DIR/setup-selfsigned-gateway-tls.sh"
tls_check_script="$SCRIPT_DIR/tls-connectivity-check.sh"
setup_akv_script="$SCRIPT_DIR/setup-documentdb-akv.sh"
provided_setup_script="$SCRIPT_DIR/documentdb-provided-mode-setup.sh"

if [[ ! -x "$create_cluster_script" || ! -x "$setup_selfsigned_script" || ! -x "$tls_check_script" || ! -x "$setup_akv_script" || ! -x "$provided_setup_script" ]]; then
  echo "Required helper scripts are missing or not executable" >&2
  exit 1
fi

run() {
  local description="$1"; shift
  echo "$(date +'%F %T') :: ${description}"
  "$@"
}

echo "Running end-to-end gateway TLS validation with:" 
echo "  Resource Group: ${RESOURCE_GROUP}"
echo "  AKS Cluster:    ${AKS_NAME}"
echo "  Location:       ${LOCATION}"
echo "  Key Vault:      ${KEYVAULT_NAME}"
echo "  Namespace:      ${NAMESPACE}"
echo "  DocumentDB:     ${DOCDB_NAME}"
echo

if [[ "$SKIP_CLUSTER" -eq 0 ]]; then
  if [[ -n "$GITHUB_USERNAME" ]]; then
    export GITHUB_USERNAME
  fi
  if [[ -n "$GITHUB_TOKEN" ]]; then
    export GITHUB_TOKEN
  fi
  run "Provision AKS cluster" bash "$create_cluster_script" \
    --cluster-name "$AKS_NAME" \
    --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" \
    --install-all
else
  run "Ensure kubeconfig for existing cluster" bash "$create_cluster_script" \
    --cluster-name "$AKS_NAME" \
    --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" \
    --skip-operator \
    --skip-instance >/dev/null
fi

run "Deploy DocumentDB self-signed mode" bash "$setup_selfsigned_script" \
  --namespace "$NAMESPACE" \
  --name "$DOCDB_NAME" \
  --skip-cert-manager

run "Validate self-signed connectivity" bash "$tls_check_script" \
  --mode selfsigned \
  --namespace "$NAMESPACE" \
  --docdb-name "$DOCDB_NAME" \
  --skip-cert-manager

SVC_IP=$(kubectl -n "$NAMESPACE" get svc "documentdb-service-${DOCDB_NAME}" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
if [[ -z "$SVC_IP" ]]; then
  echo "Failed to retrieve LoadBalancer IP for documentdb-service-${DOCDB_NAME}" >&2
  exit 1
fi
SNI_HOST="${SVC_IP}.sslip.io"
echo "Detected gateway endpoint: ${SVC_IP} (${SNI_HOST})"

run "Prepare Azure Key Vault" bash "$setup_akv_script" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --keyvault "$KEYVAULT_NAME" \
  --aks-name "$AKS_NAME" \
  --sni-host "$SNI_HOST"

KUBELET_MI_CLIENT_ID=$(az aks show -g "$RESOURCE_GROUP" -n "$AKS_NAME" --query identityProfile.kubeletidentity.clientId -o tsv)
if [[ -z "$KUBELET_MI_CLIENT_ID" ]]; then
  echo "Unable to obtain kubelet managed identity clientId" >&2
  exit 1
fi

yaml_secret_name="documentdb-provided-tls"
run "Switch cluster to provided TLS" bash "$provided_setup_script" \
  --resource-group "$RESOURCE_GROUP" \
  --aks-name "$AKS_NAME" \
  --keyvault "$KEYVAULT_NAME" \
  --cert-name documentdb-gateway \
  --sni-host "$SNI_HOST" \
  --namespace "$NAMESPACE" \
  --docdb-name "$DOCDB_NAME" \
  --provided-secret "$yaml_secret_name" \
  --user-assigned-client "$KUBELET_MI_CLIENT_ID" \
  --skip-cert-manager

run "Validate provided-mode connectivity" bash "$tls_check_script" \
  --mode provided \
  --namespace "$NAMESPACE" \
  --docdb-name "$DOCDB_NAME" \
  --provided-secret "$yaml_secret_name" \
  --sni-host "$SNI_HOST" \
  --skip-cert-manager

echo
echo "End-to-end gateway TLS validation completed successfully."