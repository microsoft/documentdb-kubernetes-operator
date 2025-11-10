#!/usr/bin/env bash

set -eu

# Define variables (allow env overrides)
RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"
# Resource Group location (does not have to match cluster regions)
RG_LOCATION="${RG_LOCATION:-eastus2}"
# Hub region
HUB_REGION="${HUB_REGION:-$RG_LOCATION}"
TEMPLATE_DIR="$(dirname "$0")"

# Regions for member clusters (keep in sync with parameters.bicepparam if you change it)
if [ -n "${MEMBER_REGIONS_CSV:-}" ]; then
  IFS=',' read -r -a MEMBER_REGIONS <<< "$MEMBER_REGIONS_CSV"
else
  MEMBER_REGIONS=("westus3" "uksouth" "eastus2")
fi

# Optional: explicitly override the VM size used by the template param hubVmSize.
# If left empty, the template's default (currently Standard_D2s_v6) will be used.
HUB_VM_SIZE="${HUB_VM_SIZE:-}"

# Build JSON arrays for parameters (after any fallbacks)
MEMBER_REGIONS_JSON=$(printf '%s\n' "${MEMBER_REGIONS[@]}" | jq -R . | jq -s .)

# Wait for any in-progress AKS operations in this resource group to finish
wait_for_no_inprogress() {
  local rg="$1"
  echo "Checking for in-progress AKS operations in resource group '$rg'..."
  # Use az aks list for reliable provisioningState at top-level
  local inprogress
  inprogress=$(az aks list -g "$rg" -o json \
    | jq -r '.[] | select(.provisioningState != "Succeeded" and .provisioningState != null) | [.name, .provisioningState] | @tsv')

  if [ -z "$inprogress" ]; then
    echo "No in-progress AKS operations detected."
    return 0
  fi

  echo "Found clusters still provisioning:" 
  echo "$inprogress" | while IFS=$'\t' read -r name state; do echo "  - $name: $state"; done
  echo "Please re-run this script after the above operations complete. To abort a stuck operation, use: az aks operation-abort --resource-group <rg> --name <cluster> --operation-id <id>" >&2
  return 1
}

echo "Creating or using resource group..."
EXISTING_RG_LOCATION=$(az group show --name "$RESOURCE_GROUP" --query location -o tsv 2>/dev/null || true)
if [ -n "$EXISTING_RG_LOCATION" ]; then
  echo "Using existing resource group '$RESOURCE_GROUP' in location '$EXISTING_RG_LOCATION'"
  RG_LOCATION="$EXISTING_RG_LOCATION"
else
  az group create --name "$RESOURCE_GROUP" --location "$RG_LOCATION"
fi

echo "Deploying AKS Fleet with Bicep..."
# Ensure we don't kick off another deployment while clusters are still provisioning
if ! wait_for_no_inprogress "$RESOURCE_GROUP"; then
  echo "Exiting without changes due to in-progress operations. Re-run when provisioning completes." >&2
  exit 1
fi
# Build parameter overrides
PARAMS=(
  --parameters "$TEMPLATE_DIR/parameters.bicepparam"
  --parameters hubRegion="$HUB_REGION"
  --parameters memberRegions="$MEMBER_REGIONS_JSON"
)
if [ -n "$HUB_VM_SIZE" ]; then
  echo "Overriding hubVmSize with: $HUB_VM_SIZE"
  PARAMS+=( --parameters hubVmSize="$HUB_VM_SIZE" )
fi

DEPLOYMENT_NAME=${DEPLOYMENT_NAME:-"aks-fleet-$(date +%s)"}
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

# Extract outputs
FLEET_NAME=$(echo $DEPLOYMENT_OUTPUT | jq -r '.fleetName.value')
FLEET_ID_FROM_OUTPUT=$(echo $DEPLOYMENT_OUTPUT | jq -r '.fleetId.value')
MEMBER_CLUSTER_NAMES=$(echo $DEPLOYMENT_OUTPUT | jq -r '.memberClusterNames.value[]')

# Get subscription ID
SUBSCRIPTION_ID=$(az account show --query id -o tsv)

# Set FLEET_ID environment variable
export FLEET_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ContainerService/fleets/${FLEET_NAME}"

# Set up RBAC access for the current user
echo "Setting up RBAC access for Fleet..."
export IDENTITY=$(az ad signed-in-user show --query "id" --output tsv)
export ROLE="Azure Kubernetes Fleet Manager RBAC Cluster Admin"
echo "Assigning role '$ROLE' to user '$IDENTITY'..."
az role assignment create --role "${ROLE}" --assignee ${IDENTITY} --scope ${FLEET_ID} >/dev/null 2>&1 || {
  echo "Note: Role assignment may already exist or you may need admin permissions to assign roles."
}

# Verify the role assignment was successful
echo "Verifying role assignment..."
ASSIGNMENT_CHECK=$(az role assignment list --assignee ${IDENTITY} --scope ${FLEET_ID} --query "[?roleDefinitionName=='${ROLE}']" -o json)
if [ "$(echo $ASSIGNMENT_CHECK | jq '. | length')" -gt 0 ]; then
  echo "✅ Role assignment verified successfully"
  echo "  Role: $ROLE"
  echo "  Assignee: $IDENTITY"
  echo "  Scope: Fleet $FLEET_NAME"
else
  echo "⚠️  WARNING: Role assignment could not be verified!"
  echo "  You may not have the required permissions to access the fleet hub."
  echo ""
  echo "  To fix this, ask an administrator to run:"
  echo "  az role assignment create --role \"${ROLE}\" --assignee ${IDENTITY} --scope ${FLEET_ID}"
  echo ""
  echo "  Or assign the role in Azure Portal:"
  echo "  1. Navigate to the Fleet resource: $FLEET_NAME"
  echo "  2. Go to Access Control (IAM)"
  echo "  3. Add role assignment"
  echo "  4. Select role: $ROLE"
  echo "  5. Assign to: $IDENTITY"
fi

# Fetch kubeconfig for hub and members to ensure contexts exist
echo "Fetching kubeconfig contexts..."
FIRST_CLUSTER=""
set +e
az fleet get-credentials --resource-group "$RESOURCE_GROUP" --name "$FLEET_NAME" --overwrite-existing 
GET_CREDS_RC=$?
set -e
if [ $GET_CREDS_RC -ne 0 ]; then
  echo "Warning: failed to get credentials for fleet hub '$FLEET_NAME'." >&2
  if [ "$(echo $ASSIGNMENT_CHECK | jq '. | length')" -eq 0 ]; then
    echo "  This is likely because the role assignment is missing (see warning above)." >&2
  fi
  echo "  Trying member clusters with admin access..." >&2
fi

while read -r cluster; do
  [ -z "$cluster" ] && continue
  set +e
  az aks get-credentials --resource-group "$RESOURCE_GROUP" --name "$cluster" --overwrite-existing >/dev/null 2>&1
  rc=$?
  set -e
  if [ $rc -eq 0 ]; then
    [ -z "$FIRST_CLUSTER" ] && FIRST_CLUSTER="$cluster"
  else
    echo "Warning: failed to get credentials for member cluster '$cluster'." >&2
  fi
done <<< "$MEMBER_CLUSTER_NAMES"

# Create kubectl aliases and export FLEET_ID (k-hub and k-<region>) persisted in ~/.bashrc
ALIASES_BLOCK_START="# BEGIN aks-fleet aliases"
ALIASES_BLOCK_END="# END aks-fleet aliases"
ALIASES_TMP=$(mktemp)
{
  echo "$ALIASES_BLOCK_START"
  echo "export FLEET_ID=\"/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ContainerService/fleets/${FLEET_NAME}\""
  echo "export IDENTITY=\"${IDENTITY}\""
  # Use first member as hub if fleet hub doesn't work
  if [ $GET_CREDS_RC -eq 0 ]; then
    echo "alias k-hub=\"kubectl --context 'hub'\""
  elif [ -n "$FIRST_CLUSTER" ]; then
    echo "alias k-hub=\"kubectl --context '$FIRST_CLUSTER'\"  # Using first member as hub"
  fi
  # For each member cluster, derive region from name pattern 'member-<region>-<suffix>' and create k-<region>
  while read -r cluster; do
    [ -z "$cluster" ] && continue
    region=$(echo "$cluster" | awk -F- '{print $2}')
    # Fallback if pattern unexpected
    [ -z "$region" ] && region="$cluster"
    echo "alias k-$region=\"kubectl --context '$cluster'\""
  done <<< "$MEMBER_CLUSTER_NAMES"
  echo "$ALIASES_BLOCK_END"
} > "$ALIASES_TMP"

BASHRC="$HOME/.bashrc"
# Create or replace block in ~/.bashrc
if [ -f "$BASHRC" ]; then
  # Remove existing block if present
  awk -v start="$ALIASES_BLOCK_START" -v end="$ALIASES_BLOCK_END" '
    $0==start {inblock=1; next}
    $0==end {inblock=0; next}
    !inblock {print}
  ' "$BASHRC" > "$BASHRC.tmp"
  cat "$ALIASES_TMP" >> "$BASHRC.tmp"
  mv "$BASHRC.tmp" "$BASHRC"
else
  cp "$ALIASES_TMP" "$BASHRC"
fi
rm -f "$ALIASES_TMP"

echo ""
echo "✅ Deployment completed successfully!"
echo ""
echo "Fleet Name: $FLEET_NAME"
echo "Fleet ID: $FLEET_ID"
echo "User Identity: $IDENTITY"
echo "RBAC Role: $ROLE"
echo "Role Assignment Status: $([ "$(echo $ASSIGNMENT_CHECK | jq '. | length')" -gt 0 ] && echo "✅ Verified" || echo "⚠️  Not verified")"
echo "Member Clusters:"
echo "$MEMBER_CLUSTER_NAMES" | while read cluster; do
  echo "  - $cluster"
done

echo ""
echo "Environment variables and aliases have been saved to ~/.bashrc:"
echo "  export FLEET_ID=$FLEET_ID"
echo "  export IDENTITY=$IDENTITY"
if [ -n "$FIRST_CLUSTER" ] && [ $GET_CREDS_RC -ne 0 ]; then
  echo "  alias k-hub points to '$FIRST_CLUSTER' (first member cluster)"
fi

echo ""
echo "To get credentials for the fleet hub (if available):"
echo "az fleet get-credentials --resource-group $RESOURCE_GROUP --name $FLEET_NAME"
echo ""
echo "If you run into login problems refer to:"
echo "https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication#azure-cli"
echo "The default is web interactive/device login which might not be allowed by your administrator."
echo "Try switching to Azure CLI in that case: kubelogin convert-kubeconfig -l azurecli"

echo ""
echo "To get credentials for member clusters with admin access:"
echo "$MEMBER_CLUSTER_NAMES" | while read cluster; do
  echo "az aks get-credentials --resource-group $RESOURCE_GROUP --name $cluster --admin"
done

echo ""
echo "Run 'source ~/.bashrc' to load the aliases and environment variables in your current session"