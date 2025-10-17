#/bin/bash

RESOURCE_GROUP="${RESOURCE_GROUP:-german-aks-fleet-rg}"
DOCUMENTDB_NAME="${DOCUMENTDB_NAME:-documentdb-preview}"
DOCUMENTDB_NAMESPACE="${DOCUMENTDB_NAMESPACE:-documentdb-preview-ns}"
HUB_CONTEXT="${HUB_CONTEXT:-hub}"
GKE_CLUSTER_NAME="${GKE_CLUSTER_NAME:-gke-documentdb-cluster}"

MEMBER_CLUSTERS=$(kubectl --context "$HUB_CONTEXT" get documentdb $DOCUMENTDB_NAME -n $DOCUMENTDB_NAMESPACE -o json | jq -r ".spec.clusterReplication.clusterList[]")
PRIMARY_CLUSTER=$(kubectl --context "$HUB_CONTEXT" get documentdb $DOCUMENTDB_NAME -n $DOCUMENTDB_NAMESPACE -o json | jq -r ".spec.clusterReplication.primary")

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

dnsName=$(az network dns zone list --resource-group $RESOURCE_GROUP --query="[0].name" -o tsv)

#delete old srv record
az network dns record-set srv remove-record \
  --record-set-name "_mongodb._tcp" \
  --zone-name "$dnsName" \
  --resource-group "$RESOURCE_GROUP" \
  --priority 0 \
  --weight 0 \
  --port 10260 \
  --target "$PRIMARY_CLUSTER.$dnsName" \
  --keep-empty-record-set

#create new one
az network dns record-set srv add-record \
  --record-set-name "_mongodb._tcp" \
  --zone-name "$dnsName" \
  --resource-group "$RESOURCE_GROUP" \
  --priority 0 \
  --weight 0 \
  --port 10260 \
  --target "$TARGET_CLUSTER.$dnsName"

echo "Initiating failover to $TARGET_CLUSTER..."
kubectl --context "$HUB_CONTEXT" patch documentdb "$DOCUMENTDB_NAME" -n "$DOCUMENTDB_NAMESPACE" \
    --type='merge' -p="{\"spec\":{\"clusterReplication\":{\"primary\":\"$TARGET_CLUSTER\"}}}"
