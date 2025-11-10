#!/usr/bin/env bash

#######################################
# DocumentDB TLS Setup - Quick Start Script
#
# This is the main entry point for creating a DocumentDB cluster with TLS support.
# It provides a simplified interface to the comprehensive gateway-tls-e2e.sh script.
#
# Usage:
#   ./create-cluster.sh --suffix myname --subscription-id <id>
#
# For full options, run:
#   ./create-cluster.sh --help
#######################################

set -euo pipefail

# Determine script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The comprehensive E2E script is in the same directory
E2E_SCRIPT="$SCRIPT_DIR/gateway-tls-e2e.sh"

# Check if the E2E script exists
if [[ ! -f "$E2E_SCRIPT" ]]; then
    echo "Error: Could not find gateway-tls-e2e.sh at: $E2E_SCRIPT"
    echo "Please ensure all scripts are present in the scripts directory"
    exit 1
fi

usage() {
    cat <<'EOF'
DocumentDB TLS Setup - Quick Start

This script creates a complete AKS cluster with DocumentDB operator and TLS support.
It handles everything from infrastructure setup to TLS validation.

USAGE:
    ./create-cluster.sh --suffix <name> --subscription-id <id> [OPTIONS]

REQUIRED:
    --suffix <value>            Unique identifier for your resources (e.g., your username)
    --subscription-id <id>      Azure subscription ID

OPTIONAL:
    --location <region>         Azure region (default: eastus2)
    --resource-group <name>     Resource group name (default: guanzhou-<suffix>-rg)
    --aks-name <name>          AKS cluster name (default: guanzhou-<suffix>)
    --keyvault <name>          Azure Key Vault name (default: ddb-issuer-<suffix>)
    --namespace <name>         Kubernetes namespace (default: documentdb-preview-ns)
    --docdb-name <name>        DocumentDB resource name (default: documentdb-preview)
    --github-username <user>   GitHub username for operator images (optional)
    --github-token <token>     GitHub token with read:packages scope (optional)
    --skip-cluster             Skip AKS cluster creation (use existing cluster)
    --help                     Show this help message

EXAMPLES:
    # Minimal setup - creates everything with defaults
    ./create-cluster.sh --suffix demo --subscription-id 12345678-1234-1234-1234-123456789012

    # Custom region and names
    ./create-cluster.sh \
        --suffix prod \
        --subscription-id 12345678-1234-1234-1234-123456789012 \
        --location westus2 \
        --resource-group my-documentdb-rg \
        --aks-name my-aks-cluster

    # Use existing AKS cluster
    ./create-cluster.sh \
        --suffix dev \
        --subscription-id 12345678-1234-1234-1234-123456789012 \
        --skip-cluster

WHAT IT DOES:
    1. ✓ Creates AKS cluster with required addons (unless --skip-cluster)
    2. ✓ Installs cert-manager and Secrets Store CSI driver
    3. ✓ Creates Azure Key Vault for certificate storage
    4. ✓ Deploys DocumentDB operator with Helm
    5. ✓ Configures SelfSigned TLS mode and validates connectivity
    6. ✓ Configures Provided TLS mode (Azure Key Vault) and validates
    7. ✓ Provides connection strings and testing instructions

TIME ESTIMATE:
    - With cluster creation: ~20-30 minutes
    - Without cluster creation: ~10-15 minutes

CLEANUP:
    To delete all resources after testing:
    ./delete-cluster.sh --suffix <your-suffix> --subscription-id <id> --all

For detailed documentation, see:
    ../README.md

For E2E testing instructions, see:
    ../E2E-TESTING.md
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
DOCDB_NAME="documentdb-preview"
SKIP_CLUSTER=0
GITHUB_USERNAME=""
GITHUB_TOKEN=""

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
        --docdb-name)
            DOCDB_NAME="$2"
            shift 2
            ;;
        --github-username)
            GITHUB_USERNAME="$2"
            shift 2
            ;;
        --github-token)
            GITHUB_TOKEN="$2"
            shift 2
            ;;
        --skip-cluster)
            SKIP_CLUSTER=1
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

# Set defaults based on suffix if not provided
if [[ -z "$RESOURCE_GROUP" ]]; then
    RESOURCE_GROUP="guanzhou-${SUFFIX}-rg"
fi

if [[ -z "$AKS_NAME" ]]; then
    AKS_NAME="guanzhou-${SUFFIX}"
fi

if [[ -z "$KEYVAULT_NAME" ]]; then
    KEYVAULT_NAME="ddb-issuer-${SUFFIX}"
fi

# Print configuration
echo "============================================"
echo "DocumentDB TLS Setup - Configuration"
echo "============================================"
echo "Suffix:          $SUFFIX"
echo "Subscription:    $SUBSCRIPTION_ID"
echo "Location:        $LOCATION"
echo "Resource Group:  $RESOURCE_GROUP"
echo "AKS Cluster:     $AKS_NAME"
echo "Key Vault:       $KEYVAULT_NAME"
echo "Namespace:       $NAMESPACE"
echo "DocumentDB:      $DOCDB_NAME"
echo "Skip Cluster:    $([ $SKIP_CLUSTER -eq 1 ] && echo 'Yes' || echo 'No')"
echo "============================================"
echo ""

# Confirm before proceeding
read -p "Proceed with this configuration? (yes/no): " -r
echo ""
if [[ ! $REPLY =~ ^[Yy]es$ ]]; then
    echo "Aborted by user."
    exit 0
fi

# Build command for the E2E script
CMD=("$E2E_SCRIPT")
CMD+=(--suffix "$SUFFIX")
CMD+=(--location "$LOCATION")
CMD+=(--resource-group "$RESOURCE_GROUP")
CMD+=(--aks-name "$AKS_NAME")
CMD+=(--keyvault "$KEYVAULT_NAME")
CMD+=(--namespace "$NAMESPACE")
CMD+=(--docdb-name "$DOCDB_NAME")

if [[ $SKIP_CLUSTER -eq 1 ]]; then
    CMD+=(--skip-cluster)
fi

if [[ -n "$GITHUB_USERNAME" ]]; then
    CMD+=(--github-username "$GITHUB_USERNAME")
fi

if [[ -n "$GITHUB_TOKEN" ]]; then
    CMD+=(--github-token "$GITHUB_TOKEN")
fi

# Export subscription ID for az commands
export AZURE_SUBSCRIPTION_ID="$SUBSCRIPTION_ID"
az account set --subscription "$SUBSCRIPTION_ID" 2>/dev/null || {
    echo "Error: Failed to set Azure subscription. Please run 'az login' first."
    exit 1
}

echo "Starting DocumentDB TLS setup..."
echo "This will take approximately 20-30 minutes..."
echo ""

# Execute the E2E script
"${CMD[@]}"

EXIT_CODE=$?

if [[ $EXIT_CODE -eq 0 ]]; then
    echo ""
    echo "============================================"
    echo "✓ DocumentDB TLS Setup Complete!"
    echo "============================================"
    echo ""
    echo "Next steps:"
    echo "1. Verify TLS status:"
    echo "   kubectl get documentdb $DOCDB_NAME -n $NAMESPACE -o jsonpath='{.status.tls}' | jq"
    echo ""
    echo "2. Get connection string:"
    echo "   kubectl get documentdb $DOCDB_NAME -n $NAMESPACE"
    echo ""
    echo "3. Test connectivity with mongosh:"
    echo "   (See output above for specific connection commands)"
    echo ""
    echo "To clean up resources:"
    echo "   ./delete-cluster.sh --suffix $SUFFIX --subscription-id $SUBSCRIPTION_ID --all"
    echo ""
else
    echo ""
    echo "============================================"
    echo "✗ Setup encountered errors"
    echo "============================================"
    echo ""
    echo "Please check the logs above for details."
    echo "For troubleshooting, see: ../README.md#troubleshooting"
    echo ""
    exit $EXIT_CODE
fi
