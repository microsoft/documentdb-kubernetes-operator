#!/usr/bin/env bash

#######################################
# DocumentDB TLS Setup - Cleanup Script
#
# This script deletes resources created by create-cluster.sh
# Supports multiple cleanup modes:
#   --all: Delete everything (cluster, RG, Key Vault)
#   --keep-cluster: Delete only DocumentDB resources
#   --keep-keyvault: Delete cluster but preserve Key Vault
#
# Usage:
#   ./delete-cluster.sh --suffix myname --subscription-id <id> --all
#######################################

set -euo pipefail

usage() {
    cat <<'EOF'
DocumentDB TLS Cleanup Script

This script deletes resources created during DocumentDB TLS setup.
You can choose to delete everything or selectively preserve certain resources.

USAGE:
    ./delete-cluster.sh --suffix <name> --subscription-id <id> [MODE] [OPTIONS]

REQUIRED:
    --suffix <value>            The suffix used during cluster creation
    --subscription-id <id>      Azure subscription ID

CLEANUP MODES (choose one):
    --all                       Delete everything: AKS, Resource Group, Key Vault, Kubernetes resources
    --keep-cluster             Delete only DocumentDB/Kubernetes resources, preserve AKS cluster
    --keep-keyvault            Delete AKS cluster but preserve Key Vault (for certificate reuse)

OPTIONAL OVERRIDES:
    --location <region>         Azure region (default: eastus2)
    --resource-group <name>     Resource group name (default: guanzhou-<suffix>-rg)
    --aks-name <name>          AKS cluster name (default: guanzhou-<suffix>)
    --keyvault <name>          Key Vault name (default: ddb-issuer-<suffix>)
    --namespace <name>         Kubernetes namespace (default: documentdb-preview-ns)
    --help                     Show this help message

EXAMPLES:
    # Delete everything (most common)
    ./delete-cluster.sh --suffix demo --subscription-id 12345678-1234-1234-1234-123456789012 --all

    # Keep cluster for reuse, delete only DocumentDB
    ./delete-cluster.sh --suffix demo --subscription-id 12345678-1234-1234-1234-123456789012 --keep-cluster

    # Delete cluster but preserve Key Vault certificates
    ./delete-cluster.sh --suffix demo --subscription-id 12345678-1234-1234-1234-123456789012 --keep-keyvault

    # Delete with custom names
    ./delete-cluster.sh \
        --suffix prod \
        --subscription-id 12345678-1234-1234-1234-123456789012 \
        --resource-group my-rg \
        --all

WHAT GETS DELETED:
    --all mode:
        ✗ DocumentDB resources (CRDs, pods, services)
        ✗ Helm releases (operator, cert-manager)
        ✗ Kubernetes namespaces
        ✗ AKS cluster
        ✗ Azure Resource Group
        ✗ Azure Key Vault

    --keep-cluster mode:
        ✗ DocumentDB resources
        ✗ Helm releases (operator only)
        ✗ Kubernetes namespaces
        ✓ AKS cluster (preserved)
        ✓ Resource Group (preserved)
        ✓ Key Vault (preserved)

    --keep-keyvault mode:
        ✗ DocumentDB resources
        ✗ Helm releases
        ✗ Kubernetes namespaces
        ✗ AKS cluster
        ✗ Resource Group
        ✓ Key Vault (preserved in new RG)

WARNINGS:
    - Deletion is permanent and cannot be undone
    - --all mode will delete the entire resource group
    - You will be prompted to confirm before deletion

EOF
}

# Default values
SUFFIX=""
SUBSCRIPTION_ID=""
LOCATION="eastus2"
RESOURCE_GROUP=""
AKS_NAME=""
KEYVAULT_NAME=""
NAMESPACE="documentdb-preview-ns"

# Cleanup modes
DELETE_ALL=0
KEEP_CLUSTER=0
KEEP_KEYVAULT=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --suffix)
            SUFFIX="$2"
            shift 2
            ;;
        --subscription-id)
            SUBSCRIPTION_ID="$2"
            shift 2
            ;;
        --location)
            LOCATION="$2"
            shift 2
            ;;
        --resource-group)
            RESOURCE_GROUP="$2"
            shift 2
            ;;
        --aks-name)
            AKS_NAME="$2"
            shift 2
            ;;
        --keyvault)
            KEYVAULT_NAME="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --all)
            DELETE_ALL=1
            shift
            ;;
        --keep-cluster)
            KEEP_CLUSTER=1
            shift
            ;;
        --keep-keyvault)
            KEEP_KEYVAULT=1
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Error: Unknown option: $1"
            echo ""
            usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [[ -z "$SUFFIX" ]]; then
    echo "Error: --suffix is required"
    echo ""
    usage
    exit 1
fi

if [[ -z "$SUBSCRIPTION_ID" ]]; then
    echo "Error: --subscription-id is required"
    echo ""
    usage
    exit 1
fi

# Validate cleanup mode
MODE_COUNT=$((DELETE_ALL + KEEP_CLUSTER + KEEP_KEYVAULT))
if [[ $MODE_COUNT -eq 0 ]]; then
    echo "Error: You must specify a cleanup mode: --all, --keep-cluster, or --keep-keyvault"
    echo ""
    usage
    exit 1
fi

if [[ $MODE_COUNT -gt 1 ]]; then
    echo "Error: Only one cleanup mode can be specified"
    echo ""
    usage
    exit 1
fi

# Set defaults based on suffix
if [[ -z "$RESOURCE_GROUP" ]]; then
    RESOURCE_GROUP="guanzhou-${SUFFIX}-rg"
fi

if [[ -z "$AKS_NAME" ]]; then
    AKS_NAME="guanzhou-${SUFFIX}"
fi

if [[ -z "$KEYVAULT_NAME" ]]; then
    KEYVAULT_NAME="ddb-issuer-${SUFFIX}"
fi

# Determine mode description
if [[ $DELETE_ALL -eq 1 ]]; then
    MODE_DESC="DELETE EVERYTHING (Cluster, Resource Group, Key Vault)"
elif [[ $KEEP_CLUSTER -eq 1 ]]; then
    MODE_DESC="Delete DocumentDB only (Keep AKS cluster)"
elif [[ $KEEP_KEYVAULT -eq 1 ]]; then
    MODE_DESC="Delete cluster (Keep Key Vault)"
fi

# Set Azure subscription
az account set --subscription "$SUBSCRIPTION_ID" 2>/dev/null || {
    echo "Error: Failed to set Azure subscription. Please run 'az login' first."
    exit 1
}

# Print configuration
echo "============================================"
echo "DocumentDB TLS Cleanup - Configuration"
echo "============================================"
echo "Mode:            $MODE_DESC"
echo "Suffix:          $SUFFIX"
echo "Subscription:    $SUBSCRIPTION_ID"
echo "Resource Group:  $RESOURCE_GROUP"
echo "AKS Cluster:     $AKS_NAME"
echo "Key Vault:       $KEYVAULT_NAME"
echo "Namespace:       $NAMESPACE"
echo "============================================"
echo ""
echo "⚠️  WARNING: This action cannot be undone!"
echo ""

# Confirm before proceeding
read -p "Are you sure you want to proceed? Type 'yes' to confirm: " -r
echo ""
if [[ ! $REPLY =~ ^yes$ ]]; then
    echo "Aborted by user."
    exit 0
fi

echo "Starting cleanup..."
echo ""

# Function to delete Kubernetes resources
delete_k8s_resources() {
    echo "→ Deleting Kubernetes resources in namespace: $NAMESPACE"
    
    # Check if cluster is accessible
    if ! kubectl cluster-info &>/dev/null; then
        echo "  ⚠️  Cannot access Kubernetes cluster, skipping K8s cleanup"
        return
    fi

    # Delete DocumentDB resources
    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
        echo "  • Deleting DocumentDB instances..."
        kubectl delete documentdb --all -n "$NAMESPACE" --ignore-not-found=true --timeout=60s || true
        
        echo "  • Deleting operator Helm release..."
        helm uninstall documentdb-operator -n "$NAMESPACE" 2>/dev/null || true
        
        echo "  • Deleting namespace..."
        kubectl delete namespace "$NAMESPACE" --timeout=60s || true
    else
        echo "  • Namespace $NAMESPACE not found, skipping"
    fi
    
    echo "  ✓ Kubernetes resources deleted"
}

# Function to delete cert-manager
delete_cert_manager() {
    echo "→ Deleting cert-manager..."
    if kubectl get namespace cert-manager &>/dev/null; then
        helm uninstall cert-manager -n cert-manager 2>/dev/null || true
        kubectl delete namespace cert-manager --timeout=60s || true
        echo "  ✓ cert-manager deleted"
    else
        echo "  • cert-manager not found, skipping"
    fi
}

# Function to move Key Vault to new resource group
preserve_keyvault() {
    echo "→ Preserving Key Vault: $KEYVAULT_NAME"
    
    # Create new resource group for Key Vault
    KV_RG="${KEYVAULT_NAME}-preserved-rg"
    
    if az keyvault show --name "$KEYVAULT_NAME" &>/dev/null; then
        echo "  • Creating new resource group: $KV_RG"
        az group create --name "$KV_RG" --location "$LOCATION" --output none
        
        echo "  • Moving Key Vault to new resource group..."
        KV_ID=$(az keyvault show --name "$KEYVAULT_NAME" --query id -o tsv)
        az resource move --destination-group "$KV_RG" --ids "$KV_ID" || {
            echo "  ⚠️  Failed to move Key Vault, it may be deleted with the resource group"
        }
        
        echo "  ✓ Key Vault preserved in: $KV_RG"
        echo "  ℹ️  To delete it later: az group delete --name $KV_RG"
    else
        echo "  • Key Vault not found, nothing to preserve"
    fi
}

# Execute cleanup based on mode
if [[ $DELETE_ALL -eq 1 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Mode: DELETE ALL"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    # Get AKS credentials for K8s cleanup
    echo "→ Getting AKS credentials..."
    az aks get-credentials --resource-group "$RESOURCE_GROUP" --name "$AKS_NAME" --overwrite-existing 2>/dev/null || {
        echo "  ⚠️  Could not get AKS credentials, cluster may not exist"
    }
    
    delete_k8s_resources
    
    echo ""
    echo "→ Deleting Azure Resource Group: $RESOURCE_GROUP"
    echo "  (This includes AKS cluster, Key Vault, and all other resources)"
    
    if az group show --name "$RESOURCE_GROUP" &>/dev/null; then
        az group delete --name "$RESOURCE_GROUP" --yes --no-wait
        echo "  ✓ Resource group deletion initiated (running in background)"
        echo "  ℹ️  Check status with: az group show --name $RESOURCE_GROUP"
    else
        echo "  • Resource group not found"
    fi

elif [[ $KEEP_CLUSTER -eq 1 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Mode: KEEP CLUSTER"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    # Get AKS credentials
    echo "→ Getting AKS credentials..."
    az aks get-credentials --resource-group "$RESOURCE_GROUP" --name "$AKS_NAME" --overwrite-existing || {
        echo "Error: Could not get AKS credentials"
        exit 1
    }
    
    delete_k8s_resources
    
    echo ""
    echo "  ✓ Cleanup complete"
    echo "  ℹ️  AKS cluster preserved: $AKS_NAME"
    echo "  ℹ️  Resource group preserved: $RESOURCE_GROUP"
    echo "  ℹ️  Key Vault preserved: $KEYVAULT_NAME"

elif [[ $KEEP_KEYVAULT -eq 1 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Mode: KEEP KEY VAULT"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    # Get AKS credentials
    echo "→ Getting AKS credentials..."
    az aks get-credentials --resource-group "$RESOURCE_GROUP" --name "$AKS_NAME" --overwrite-existing 2>/dev/null || {
        echo "  ⚠️  Could not get AKS credentials"
    }
    
    delete_k8s_resources
    
    echo ""
    preserve_keyvault
    
    echo ""
    echo "→ Deleting Azure Resource Group: $RESOURCE_GROUP"
    
    if az group show --name "$RESOURCE_GROUP" &>/dev/null; then
        az group delete --name "$RESOURCE_GROUP" --yes --no-wait
        echo "  ✓ Resource group deletion initiated (running in background)"
    else
        echo "  • Resource group not found"
    fi
fi

echo ""
echo "============================================"
echo "✓ Cleanup Complete"
echo "============================================"
echo ""

if [[ $DELETE_ALL -eq 1 ]]; then
    echo "All resources have been deleted or are being deleted."
    echo "Deletion may take several minutes to complete."
elif [[ $KEEP_CLUSTER -eq 1 ]]; then
    echo "DocumentDB resources deleted. AKS cluster is ready for reuse."
    echo ""
    echo "To redeploy DocumentDB:"
    echo "  ./create-cluster.sh --suffix $SUFFIX --subscription-id $SUBSCRIPTION_ID --skip-cluster"
elif [[ $KEEP_KEYVAULT -eq 1 ]]; then
    echo "Cluster deleted. Key Vault preserved for certificate reuse."
    echo ""
    echo "Key Vault location: $KV_RG"
fi

echo ""
