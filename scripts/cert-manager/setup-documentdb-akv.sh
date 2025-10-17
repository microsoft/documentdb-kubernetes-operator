#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: setup-documentdb-akv.sh [options]

Idempotently prepares Azure Key Vault and RBAC prerequisites for DocumentDB
Provided TLS mode. The script can create the resource group and Key Vault,
assign the required roles to the current user and AKS kubelet identity, and
issue a self-signed certificate with the desired SNI host.

Options:
  -g, --resource-group <name>   Azure resource group to host the Key Vault (required)
  -l, --location <region>       Azure location (required when creating the resource group or Key Vault)
      --subscription <id>       Azure subscription ID (optional; defaults to current)
      --keyvault <name>         Azure Key Vault name (required)
      --aks-name <name>         AKS cluster name for kubelet identity (required)
      --cert-name <name>        Certificate name in Key Vault (default: documentdb-gateway)
      --sni-host <host>         Hostname for certificate CN/SAN (required)
  --human-object-id <id>    Object ID to grant Key Vault Certificates Officer (default: signed-in user)
  --human-principal-type <type> Principal type for the human assignment (default: User)
  --kubelet-object-id <id>  Object ID to grant Key Vault Secrets User (default: derived from AKS)
  --kubelet-principal-type <type> Principal type for the kubelet assignment (default: ServicePrincipal)
      --validity-months <n>     Certificate validity in months (default: 12)
      --skip-certificate        Skip certificate creation (if managed externally)
  -h, --help                    Show this help message
EOF
}

RESOURCE_GROUP=""
LOCATION=""
SUBSCRIPTION_ID=""
KEYVAULT_NAME=""
AKS_NAME=""
CERT_NAME="documentdb-gateway"
SNI_HOST=""
HUMAN_OBJECT_ID=""
HUMAN_PRINCIPAL_TYPE="User"
KUBELET_OBJECT_ID=""
KUBELET_PRINCIPAL_TYPE="ServicePrincipal"
VALIDITY_MONTHS=12
CREATE_CERT=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    -g|--resource-group)
      RESOURCE_GROUP="$2"; shift 2 ;;
    -l|--location)
      LOCATION="$2"; shift 2 ;;
    --subscription)
      SUBSCRIPTION_ID="$2"; shift 2 ;;
    --keyvault)
      KEYVAULT_NAME="$2"; shift 2 ;;
    --aks-name)
      AKS_NAME="$2"; shift 2 ;;
    --cert-name)
      CERT_NAME="$2"; shift 2 ;;
    --sni-host)
      SNI_HOST="$2"; shift 2 ;;
    --human-object-id)
      HUMAN_OBJECT_ID="$2"; shift 2 ;;
    --human-principal-type)
      HUMAN_PRINCIPAL_TYPE="$2"; shift 2 ;;
    --kubelet-object-id)
      KUBELET_OBJECT_ID="$2"; shift 2 ;;
    --kubelet-principal-type)
      KUBELET_PRINCIPAL_TYPE="$2"; shift 2 ;;
    --validity-months)
      VALIDITY_MONTHS="$2"; shift 2 ;;
    --skip-certificate)
      CREATE_CERT=0; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage; exit 1 ;;
  esac
done

if [[ -z "$RESOURCE_GROUP" || -z "$KEYVAULT_NAME" || -z "$AKS_NAME" || -z "$SNI_HOST" ]]; then
  echo "Missing required arguments." >&2
  usage
  exit 1
fi

sanitize_id() {
  printf '%s' "$1" | tr -d '\r\n'
}

for cmd in az jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Command '$cmd' not found in PATH" >&2
    exit 1
  fi
done

if ! az account show >/dev/null 2>&1; then
  echo "Azure CLI not logged in. Run 'az login' first." >&2
  exit 1
fi

if [[ -n "$SUBSCRIPTION_ID" ]]; then
  az account set --subscription "$SUBSCRIPTION_ID" >/dev/null
fi
SUBSCRIPTION_ID=$(sanitize_id "$(az account show --query id -o tsv)")

ensure_resource_group() {
  if az group show -n "$RESOURCE_GROUP" >/dev/null 2>&1; then
    echo "Resource group $RESOURCE_GROUP already exists"
  else
    if [[ -z "$LOCATION" ]]; then
      echo "Resource group $RESOURCE_GROUP not found and --location not provided" >&2
      exit 1
    fi
    echo "Creating resource group $RESOURCE_GROUP in $LOCATION"
    az group create -n "$RESOURCE_GROUP" -l "$LOCATION" >/dev/null
  fi
}

ensure_key_vault() {
  if az keyvault show -n "$KEYVAULT_NAME" -g "$RESOURCE_GROUP" >/dev/null 2>&1; then
    echo "Key Vault $KEYVAULT_NAME already exists"
  else
    if [[ -z "$LOCATION" ]]; then
      echo "Key Vault $KEYVAULT_NAME not found and --location not provided" >&2
      exit 1
    fi
    echo "Creating Key Vault $KEYVAULT_NAME in $LOCATION"
    az keyvault create -g "$RESOURCE_GROUP" -n "$KEYVAULT_NAME" -l "$LOCATION" --enable-rbac-authorization true >/dev/null
  fi
}

resolve_object_ids() {
  if [[ -z "$HUMAN_OBJECT_ID" ]]; then
    HUMAN_OBJECT_ID=$(az ad signed-in-user show --query id -o tsv)
  fi
  HUMAN_OBJECT_ID=$(sanitize_id "$HUMAN_OBJECT_ID")
  echo "Using signed-in user objectId $HUMAN_OBJECT_ID for certificates officer role"
  if [[ -z "$KUBELET_OBJECT_ID" ]]; then
    KUBELET_OBJECT_ID=$(az aks show -g "$RESOURCE_GROUP" -n "$AKS_NAME" --query identityProfile.kubeletidentity.objectId -o tsv)
  fi
  KUBELET_OBJECT_ID=$(sanitize_id "$KUBELET_OBJECT_ID")
  echo "Derived kubelet objectId $KUBELET_OBJECT_ID from AKS cluster"
}

ensure_role_assignment() {
  local ASSIGNEE="$1"
  local ROLE_NAME="$2"
  local PRINCIPAL_TYPE="$3"
  ASSIGNEE=$(sanitize_id "$ASSIGNEE")
  local SCOPE="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.KeyVault/vaults/${KEYVAULT_NAME}"
  if az role assignment list --assignee-object-id "$ASSIGNEE" --role "$ROLE_NAME" --scope "$SCOPE" --query '[0]' -o tsv 2>/dev/null | grep -q '.'; then
    echo "Role $ROLE_NAME already assigned to $ASSIGNEE"
  else
    echo "Assigning $ROLE_NAME to $ASSIGNEE"
    az role assignment create --assignee-object-id "$ASSIGNEE" --assignee-principal-type "$PRINCIPAL_TYPE" --role "$ROLE_NAME" --scope "$SCOPE" >/dev/null
  fi
}

ensure_certificate() {
  if [[ "$CREATE_CERT" -eq 0 ]]; then
    echo "Skipping certificate creation as requested"
    return
  fi
  if az keyvault certificate show --vault-name "$KEYVAULT_NAME" -n "$CERT_NAME" >/dev/null 2>&1; then
    echo "Certificate $CERT_NAME already exists in Key Vault"
    return
  fi
  echo "Creating self-signed certificate $CERT_NAME with subject $SNI_HOST"
  POLICY_FILE=$(mktemp)
  cat <<EOF > "$POLICY_FILE"
{
  "issuerParameters": { "name": "Self" },
  "x509CertificateProperties": {
    "subject": "CN=${SNI_HOST}",
    "subjectAlternativeNames": { "dnsNames": [ "${SNI_HOST}" ] },
    "keyUsage": [ "digitalSignature", "keyEncipherment" ],
    "validityInMonths": ${VALIDITY_MONTHS}
  },
  "keyProperties": {
    "exportable": true,
    "keyType": "RSA",
    "keySize": 2048,
    "reuseKey": false
  },
  "secretProperties": { "contentType": "application/x-pem-file" }
}
EOF
  az keyvault certificate create --vault-name "$KEYVAULT_NAME" -n "$CERT_NAME" --policy @"$POLICY_FILE" >/dev/null
  rm -f "$POLICY_FILE"
}

ensure_resource_group
ensure_key_vault
resolve_object_ids
ensure_role_assignment "$HUMAN_OBJECT_ID" "Key Vault Certificates Officer" "$HUMAN_PRINCIPAL_TYPE"
ensure_role_assignment "$KUBELET_OBJECT_ID" "Key Vault Secrets User" "$KUBELET_PRINCIPAL_TYPE"
ensure_certificate

echo "Azure Key Vault setup complete."
