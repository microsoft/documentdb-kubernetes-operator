#/bin/bash

RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"
DOCUMENTDB_NAME="${DOCUMENTDB_NAME:-documentdb-preview}"
DOCUMENTDB_NAMESPACE="${DOCUMENTDB_NAMESPACE:-documentdb-preview-ns}"
TRAFFIC_MANAGER_PROFILE_NAME="${TRAFFIC_MANAGER_PROFILE_NAME:-${RESOURCE_GROUP}-documentdb-tm}"
HUB_CONTEXT="${HUB_CONTEXT:-hub}"

# Get all clusters
echo "Discovering member clusters in resource group: $RESOURCE_GROUP..."
MEMBER_CLUSTERS=$(az aks list -g "$RESOURCE_GROUP" -o json | jq -r '.[] | select(.name|startswith("member-")) | .name' | sort)

if [ -z "$MEMBER_CLUSTERS" ]; then
  echo "Error: No member clusters found in resource group $RESOURCE_GROUP"
  echo "Please ensure the fleet is deployed first using ./deploy-fleet-bicep.sh"
  exit 1
fi

PRIMARY_CLUSTER=$(kubectl get documentdb $DOCUMENTDB_NAME -n $DOCUMENTDB_NAMESPACE -o json | jq ".spec.clusterReplication.primary")

# Convert to array
CLUSTER_ARRAY=($MEMBER_CLUSTERS)
echo "Found ${#CLUSTER_ARRAY[@]} member clusters:"
for cluster in "${CLUSTER_ARRAY[@]}"; do
  echo "  - $cluster"
  if [ "$cluster" == "$PRIMARY_CLUSTER" ]; then
    echo "    (current primary)"
  else 
    TARGET_CLUSTER="$cluster"
  fi
done

echo "Updating Traffic Manager to point to new primary: $TARGET_CLUSTER..."

# Find the lowest priority not in use
PRIORITIES=$(az network traffic-manager profile show \
    --resource-group "$RESOURCE_GROUP" \
    --profile-name "$TRAFFIC_MANAGER_PROFILE_NAME" \
    | jq ".endpoints[].priority" \
    | sort -n)
LOWEST_AVAILABLE_PRIORITY=1
for x in $PRIORITIES; do
  if [ "$x" = "$LOWEST_AVAILABLE_PRIORITY" ]; then
    LOWEST_AVAILABLE_PRIORITY=$((LOWEST_AVAILABLE_PRIORITY + 1))
  else
    break
  fi
done


PRIMARY_REGION=$(echo "$PRIMARY_CLUSTER" | awk -F- '{print $2}')
TARGET_REGION=$(echo "$TARGET_CLUSTER" | awk -F- '{print $2}')

# Set the old primary to that priority, set the target to 1
az network traffic-manager endpoint update \
    --type externalEndpoints \
    --resource-group "$RESOURCE_GROUP" \
    --profile-name "$TRAFFIC_MANAGER_PROFILE_NAME" \
    --name "documentdb-$PRIMARY_REGION" \
    --priority $LOWEST_AVAILABLE_PRIORITY

az network traffic-manager endpoint update \
    --type externalEndpoints \
    --resource-group "$RESOURCE_GROUP" \
    --profile-name "$TRAFFIC_MANAGER_PROFILE_NAME" \
    --name "documentdb-$TARGET_REGION" \
    --priority 1

echo "Initiating failover to $TARGET_CLUSTER..."
kubectl --context "$HUB_CONTEXT" patch documentdb "$DOCUMENTDB_NAME" -n "$DOCUMENTDB_NAMESPACE" \
    --type='merge' -p="{\"spec\":{\"clusterReplication\":{\"primary\":\"$TARGET_CLUSTER\"}}}"
