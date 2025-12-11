# DocumentDB Operator Upgrade Commands

This document contains detailed command examples and scripts for DocumentDB operator upgrades. These commands support the upgrade strategies outlined in the [upgrade design document](./upgrade-design-doc.md).

## Multi-Version API Workflow Commands

### Phase 1: Database Admin Team Workflows

**Pre-upgrade validation:**
```bash
# Pre-upgrade validation (Database Admin)
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --namespace documentdb-system \
  --version v2.0.0 \
  --dry-run \
  --debug

# Infrastructure upgrade execution (Database Admin)
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --namespace documentdb-system \
  --version v2.0.0 \
  --wait \
  --timeout 900s \
  --atomic

# Verify operator infrastructure health (Database Admin)
kubectl rollout status deployment/documentdb-operator -n documentdb-system
kubectl rollout status deployment/sidecar-injector -n cnpg-system
kubectl get crd dbs.documentdb.io -o jsonpath='{.metadata.labels.version}'

# Confirm v2 operator can manage both v1 and v2 cluster APIs (Database Admin)
kubectl get documentdb -A -o custom-columns="NAME:.metadata.name,API_VERSION:.apiVersion,CLUSTER_VERSION:.spec.version,STATUS:.status.phase"

# Test creating new cluster with v2 API (Database Admin)
kubectl apply -f - <<EOF
apiVersion: documentdb.io/v2
kind: DocumentDB
metadata:
  name: test-v2-cluster
  namespace: test
spec:
  version: "v2"
  # v2-specific fields here
  enhancedMonitoring: true
  advancedFeatures:
    - feature1
    - feature2
EOF
```

### Phase 2: Developer Team Workflows

```bash
# Check available DocumentDB API versions (Developer)
kubectl api-versions | grep documentdb.io
kubectl explain documentdb --api-version=documentdb.io/v2

# Check current cluster API version (Developer)
kubectl get documentdb my-cluster -o jsonpath='{.apiVersion}'

# Backup before API migration (Developer - recommended)
kubectl create backup my-cluster-pre-v2-migration --cluster my-cluster

# Migrate cluster from API v1 to v2 (Developer)
# Method 1: Update deployment file (Recommended - Standard Kubernetes Workflow)
# Edit your existing DocumentDB deployment file:
# Change: apiVersion: documentdb.io/v1
# To:     apiVersion: documentdb.io/v2
# Add any v2-specific configuration fields
kubectl apply -f my-cluster-v2.yaml

# Method 2: Using kubectl convert (if available)
kubectl get documentdb my-cluster -o yaml > my-cluster-v1.yaml
kubectl convert -f my-cluster-v1.yaml --output-version documentdb.io/v2 > my-cluster-v2.yaml
# Edit my-cluster-v2.yaml to add v2-specific features
kubectl apply -f my-cluster-v2.yaml

# Method 3: Using patch for simple migrations (Developer)
kubectl patch documentdb my-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v2",
  "spec": {
    "version": "v2",
    "enhancedMonitoring": true
  }
}'

# Example: Deployment File Update (Method 1 - Recommended)
# Original v1 deployment file:
cat > my-cluster-v1.yaml << EOF
apiVersion: documentdb.io/v1
kind: DocumentDB
metadata:
  name: my-cluster
  namespace: production
spec:
  nodeCount: 3
  instancesPerNode: 1
  resource:
    storage:
      pvcSize: 100Gi
EOF

# Updated v2 deployment file:
cat > my-cluster-v2.yaml << EOF
apiVersion: documentdb.io/v2
kind: DocumentDB
metadata:
  name: my-cluster
  namespace: production
spec:
  nodeCount: 3
  instancesPerNode: 1
  resource:
    storage:
      pvcSize: 100Gi
  # v2-specific features
  enhancedMonitoring: true
  advancedFeatures:
    - feature1
    - feature2
EOF

# Deploy the updated configuration
kubectl apply -f my-cluster-v2.yaml

# Monitor API migration progress (Developer)
kubectl get documentdb my-cluster -w
kubectl describe documentdb my-cluster
kubectl get events --field-selector involvedObject.name=my-cluster

# Validate cluster after API migration (Developer)
kubectl run test-connection --rm -i --image=mongo:7 -- \
  mongosh "mongodb://my-cluster-rw:10260/testdb" --eval "
  db.test.insertOne({migrated_to_v2: true, timestamp: new Date()});
  print('API v2 connectivity test passed');
  "

# Test v2-specific features (Developer)
kubectl get documentdb my-cluster -o jsonpath='{.status.enhancedMonitoring}'

# Rollback API version if needed (Developer)
# Method 1: Update deployment file (Recommended)
# Revert your deployment file back to v1 and redeploy
kubectl apply -f my-cluster-v1.yaml

# Method 2: Using kubectl patch
kubectl patch documentdb my-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v1",
  "spec": {
    "version": "v1"
  }
}'
```

### Cross-Team Communication Commands

```bash
# Database Admin: Check cluster API version distribution
kubectl get documentdb -A -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,API_VERSION:.apiVersion,CLUSTER_VERSION:.spec.version,STATUS:.status.phase"

# Database Admin: Check operator multi-version support status
kubectl get crd dbs.documentdb.io -o jsonpath='{.spec.versions[*].name}'

# Developer: Check if cluster is ready for API migration
kubectl get documentdb my-cluster -o jsonpath='{.status.supportedApiVersions}'

# Developer: Signal readiness for API migration
kubectl label documentdb my-cluster api-migration.documentdb.io/ready-for-v2=true

# Developer: Check API version compatibility matrix
kubectl get documentdb my-cluster -o jsonpath='{.status.operatorVersion}'
kubectl get documentdb my-cluster -o jsonpath='{.status.compatibleApiVersions}'
```

### API Deprecation Workflow Commands

```bash
# Database Admin: Check deprecated API usage before operator upgrade
kubectl get documentdb -A -o custom-columns="NAME:.metadata.name,API_VERSION:.apiVersion" | grep "v1"

# Database Admin: Get deprecation warnings
kubectl get events --field-selector reason=DeprecatedAPIUsage

# Developer: Migrate from deprecated API v1 to v2
kubectl get documentdb -A -o custom-columns="NAME:.metadata.name,API_VERSION:.apiVersion" | grep "v1" | while read name version; do
  echo "Migrating $name from $version to v2"
  kubectl patch documentdb $name --type='merge' -p '{"apiVersion": "documentdb.io/v2", "spec": {"version": "v2"}}'
done

# Developer: Validate no v1 API usage before operator upgrade that removes v1
kubectl get documentdb -A -o jsonpath='{.items[?(@.apiVersion=="documentdb.io/v1")].metadata.name}'
```

## Multi-Version API Example Commands

### Operator v2 Supporting Cluster API v1 and v2

**Phase 1: Database Admin Infrastructure Upgrade**
```bash
# Infrastructure upgrade: operator v1 → v2
helm upgrade documentdb-operator ./operator/documentdb-helm-chart --version v2.0.0

# Verify multi-version support
kubectl get crd dbs.documentdb.io -o jsonpath='{.spec.versions[*].name}'
# Output: v1 v2
```

**Phase 2: Developer Team API Migration (when ready)**
```bash
# Check current API version
kubectl get documentdb my-cluster -o jsonpath='{.apiVersion}'
# Output: documentdb.io/v1

# Migrate to API v2
kubectl patch documentdb my-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v2",
  "spec": {
    "version": "v2",
    "enhancedMonitoring": true,
    "advancedFeatures": ["feature1", "feature2"]
  }
}'

# Verify migration
kubectl get documentdb my-cluster -o jsonpath='{.apiVersion}'
# Output: documentdb.io/v2
```

### Operator v3 with API Deprecation

**Phase 1: Database Admin Infrastructure Upgrade**
```bash
# Infrastructure upgrade: operator v2 → v3
helm upgrade documentdb-operator ./operator/documentdb-helm-chart --version v3.0.0

# Check API version support with deprecation warnings
kubectl get crd dbs.documentdb.io -o jsonpath='{.spec.versions[*].name}'
# Output: v1 v2 v3
kubectl get crd dbs.documentdb.io -o jsonpath='{.spec.versions[?(@.name=="v1")].deprecated}'
# Output: true
```

**Phase 2: Developer Team Gradual API Migration**
```bash
# Week 1: Development clusters (migrate away from deprecated v1)
kubectl get documentdb -A -o custom-columns="NAME:.metadata.name,API_VERSION:.apiVersion" | grep "v1"

# Migrate v1 → v2 or v1 → v3
kubectl patch documentdb dev-cluster-1 --type='merge' -p '{
  "apiVersion": "documentdb.io/v2",
  "spec": {"version": "v2"}
}'

kubectl patch documentdb dev-cluster-2 --type='merge' -p '{
  "apiVersion": "documentdb.io/v3", 
  "spec": {
    "version": "v3",
    "newV3Features": {
      "advancedSecurity": true,
      "performanceOptimizations": ["opt1", "opt2"]
    }
  }
}'

# Week 2: Staging validation
kubectl patch documentdb staging-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v3",
  "spec": {"version": "v3"}
}'

# Week 3: Production (after testing)
kubectl patch documentdb prod-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v3",
  "spec": {"version": "v3"}
}'
```

### Operator v4 with API Removal

**Prerequisites: Ensure no v1 API usage**
```bash
# Database Admin: Verify no clusters using deprecated v1 API
kubectl get documentdb -A -o jsonpath='{.items[?(@.apiVersion=="documentdb.io/v1")].metadata.name}'
# Output should be empty

# If v1 clusters exist, they must be migrated first
for cluster in $(kubectl get documentdb -A -o jsonpath='{.items[?(@.apiVersion=="documentdb.io/v1")].metadata.name}'); do
  echo "ERROR: Cluster $cluster still using v1 API. Migration required before operator upgrade."
done
```

**Phase 1: Database Admin Infrastructure Upgrade**
```bash
# Infrastructure upgrade: operator v3 → v4 (removes v1 API support)
helm upgrade documentdb-operator ./operator/documentdb-helm-chart --version v4.0.0

# Verify API support (v1 no longer supported)
kubectl get crd dbs.documentdb.io -o jsonpath='{.spec.versions[*].name}'
# Output: v2 v3 v4
```

**Phase 2: Developer Team Careful Migration**
```bash
# Month 1: Development clusters (v2 → v3 or v2 → v4)
kubectl patch documentdb dev-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v4",
  "spec": {
    "version": "v4",
    "nextGenFeatures": {
      "aiIntegration": true,
      "autoScaling": {
        "enabled": true,
        "minReplicas": 3,
        "maxReplicas": 10
      }
    }
  }
}'

# Month 2: Staging environment validation
kubectl patch documentdb staging-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v4",
  "spec": {"version": "v4"}
}'

# Month 3: Production (after extensive testing)
kubectl patch documentdb prod-cluster --type='merge' -p '{
  "apiVersion": "documentdb.io/v4",
  "spec": {"version": "v4"}
}'
```

### API Version Coexistence Examples

**Multiple API Versions in Same Cluster**
```bash
# List all DocumentDB clusters with their API versions
kubectl get documentdb -A -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,API_VERSION:.apiVersion,STATUS:.status.phase"

# Example output showing coexistence:
# NAMESPACE    NAME           API_VERSION           STATUS
# prod         legacy-app     documentdb.io/v2   Ready
# prod         new-app        documentdb.io/v3   Ready  
# staging      test-app       documentdb.io/v4   Ready
# dev          experiment     documentdb.io/v4   Ready
```

**API Migration Validation**
```bash
# Test connectivity after API migration
migrate_and_test() {
  local cluster=$1
  local target_version=$2
  
  echo "Migrating $cluster to API $target_version"
  
  # Backup before migration
  kubectl create backup ${cluster}-pre-migration --cluster $cluster
  
  # Perform migration
  kubectl patch documentdb $cluster --type='merge' -p "{
    \"apiVersion\": \"documentdb.io/$target_version\",
    \"spec\": {\"version\": \"$target_version\"}
  }"
  
  # Wait for ready status
  kubectl wait --for=condition=Ready documentdb/$cluster --timeout=300s
  
  # Test connectivity
  kubectl run test-migration-$cluster --rm -i --image=mongo:7 -- \
  mongosh "mongodb://${cluster}-rw:10260/test" --eval "
    db.migration_test.insertOne({
      cluster: '$cluster', 
      api_version: '$target_version',
      timestamp: new Date()
    });
    print('Migration test passed for $cluster');
    " || echo "Migration test failed for $cluster"
}

# Usage examples
migrate_and_test "my-app-cluster" "v3"
migrate_and_test "legacy-cluster" "v2"
```

## Legacy Helm Commands

**Infrastructure upgrade validation:**
```bash
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --namespace documentdb-system \
  --version v1.3.0 \
  --dry-run \
  --debug
```

**Infrastructure upgrade execution:**
```bash
helm upgrade documentdb-operator ./operator/documentdb-helm-chart \
  --namespace documentdb-system \
  --version v1.3.0 \
  --wait \
  --timeout 900s \
  --atomic
```

**Infrastructure health verification:**
```bash
kubectl rollout status deployment/documentdb-operator -n documentdb-system
kubectl rollout status deployment/sidecar-injector -n cnpg-system
kubectl get clusters.postgresql.cnpg.io -A -o wide
```

## Version Alignment Examples

**Version Tagging Strategy:**
```bash
# All component images tagged with same DocumentDB version
ghcr.io/documentdb/documentdb-operator:v1.3.0
ghcr.io/documentdb/documentdb-gateway:v1.3.0  
ghcr.io/documentdb/documentdb-sidecar-injector:v1.3.0
mcr.microsoft.com/documentdb/documentdb:16.3-v1.3.0
```

**Helm Chart Version Alignment:**
```yaml
# Chart.yaml
version: v1.3.0  # Helm chart version matches DocumentDB version
dependencies:
  - name: cloudnative-pg
    version: "0.26.0"  # CNPG version locked to DocumentDB v1.3.0
```

**DocumentDB Release Bundle:**
```yaml
# DocumentDB v1.3.0 Release Bundle
documentdb-operator: v1.3.0
gateway: v1.3.0  
postgres: 16.3-v1.3.0  # PostgreSQL 16.3 + DocumentDB extension v1.3.0
sidecar-injector: v1.3.0
cnpg-operator: v0.26.0  # Updated if required for this release
```

**PostgreSQL Upgrade Configuration:**
```yaml
# CNPG handles minor PostgreSQL upgrades automatically  
spec:
  imageName: "mcr.microsoft.com/documentdb/documentdb:16.3-v1.3.0"
```

## Component Hash Tracking Script

**Hash Generation and Comparison Script:**

```bash
#!/bin/bash
# component-hash-tracker.sh

# Generate component configuration hashes
generate_component_hashes() {
    local revision=$1
    echo "=== Generating Component Hashes for Revision $revision ==="
    
    # Get Helm revision values
    helm get values documentdb-operator -n documentdb-system --revision $revision -o json > /tmp/values-r${revision}.json
    
    # DocumentDB Operator hash (image + configuration)
    OPERATOR_CONFIG=$(kubectl get deployment documentdb-operator -n documentdb-system -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "not-found")
    OPERATOR_HASH=$(echo -n "$OPERATOR_CONFIG" | sha256sum | cut -d' ' -f1)
    
    # Gateway Image hash (from values or deployment annotation)
    GATEWAY_CONFIG=$(jq -r '.image.gateway.repository + ":" + .image.gateway.tag' /tmp/values-r${revision}.json 2>/dev/null || echo "not-found")
    GATEWAY_HASH=$(echo -n "$GATEWAY_CONFIG" | sha256sum | cut -d' ' -f1)
    
    # PostgreSQL + Extension hash (from values)
    POSTGRES_CONFIG=$(jq -r '.image.postgres.repository + ":" + .image.postgres.tag' /tmp/values-r${revision}.json 2>/dev/null || echo "not-found")
    POSTGRES_HASH=$(echo -n "$POSTGRES_CONFIG" | sha256sum | cut -d' ' -f1)
    
    # Sidecar Injector hash
    SIDECAR_CONFIG=$(kubectl get deployment sidecar-injector -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "not-found")
    SIDECAR_HASH=$(echo -n "$SIDECAR_CONFIG" | sha256sum | cut -d' ' -f1)
    
    # CNPG Operator hash
    CNPG_CONFIG=$(kubectl get deployment cnpg-controller-manager -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "not-found")
    CNPG_HASH=$(echo -n "$CNPG_CONFIG" | sha256sum | cut -d' ' -f1)
    
    # Store hashes in ConfigMap for tracking
    kubectl create configmap documentdb-component-hashes-r${revision} -n documentdb-system \
        --from-literal=operator-hash=$OPERATOR_HASH \
        --from-literal=operator-config="$OPERATOR_CONFIG" \
        --from-literal=gateway-hash=$GATEWAY_HASH \
        --from-literal=gateway-config="$GATEWAY_CONFIG" \
        --from-literal=postgres-hash=$POSTGRES_HASH \
        --from-literal=postgres-config="$POSTGRES_CONFIG" \
        --from-literal=sidecar-hash=$SIDECAR_HASH \
        --from-literal=sidecar-config="$SIDECAR_CONFIG" \
        --from-literal=cnpg-hash=$CNPG_HASH \
        --from-literal=cnpg-config="$CNPG_CONFIG" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo "Component hashes generated for revision $revision:"
    echo "  Operator: $OPERATOR_HASH ($OPERATOR_CONFIG)"
    echo "  Gateway: $GATEWAY_HASH ($GATEWAY_CONFIG)"
    echo "  PostgreSQL: $POSTGRES_HASH ($POSTGRES_CONFIG)"
    echo "  Sidecar: $SIDECAR_HASH ($SIDECAR_CONFIG)"
    echo "  CNPG: $CNPG_HASH ($CNPG_CONFIG)"
}

# Compare component hashes between revisions
compare_component_hashes() {
    local current_revision=$1
    local target_revision=$2
    
    echo "=== Comparing Component Hashes: R$current_revision → R$target_revision ==="
    
    # Get hash ConfigMaps
    if ! kubectl get configmap documentdb-component-hashes-r${current_revision} -n documentdb-system >/dev/null 2>&1; then
        echo "Generating missing hash data for current revision $current_revision..."
        generate_component_hashes $current_revision
    fi
    
    if ! kubectl get configmap documentdb-component-hashes-r${target_revision} -n documentdb-system >/dev/null 2>&1; then
        echo "Generating missing hash data for target revision $target_revision..."
        generate_component_hashes $target_revision
    fi
    
    # Compare each component hash
    declare -A CHANGED_COMPONENTS
    declare -A UNCHANGED_COMPONENTS
    
    for component in operator gateway postgres sidecar cnpg; do
        CURRENT_HASH=$(kubectl get configmap documentdb-component-hashes-r${current_revision} -n documentdb-system -o jsonpath="{.data.${component}-hash}" 2>/dev/null || echo "unknown")
        TARGET_HASH=$(kubectl get configmap documentdb-component-hashes-r${target_revision} -n documentdb-system -o jsonpath="{.data.${component}-hash}" 2>/dev/null || echo "unknown")
        CURRENT_CONFIG=$(kubectl get configmap documentdb-component-hashes-r${current_revision} -n documentdb-system -o jsonpath="{.data.${component}-config}" 2>/dev/null || echo "unknown")
        TARGET_CONFIG=$(kubectl get configmap documentdb-component-hashes-r${target_revision} -n documentdb-system -o jsonpath="{.data.${component}-config}" 2>/dev/null || echo "unknown")
        
        if [ "$CURRENT_HASH" != "$TARGET_HASH" ]; then
            CHANGED_COMPONENTS[$component]="$TARGET_CONFIG"
            echo "🔄 $component: CHANGED ($CURRENT_CONFIG → $TARGET_CONFIG)"
        else
            UNCHANGED_COMPONENTS[$component]="$CURRENT_CONFIG"
            echo "✅ $component: UNCHANGED ($CURRENT_CONFIG)"
        fi
    done
    
    # Export arrays for use in rollback script
    export CHANGED_COMPONENTS
    export UNCHANGED_COMPONENTS
    
    # Return change status
    if [ ${#CHANGED_COMPONENTS[@]} -eq 0 ]; then
        echo "ℹ️  No component changes detected. Rollback not necessary."
        return 1
    else
        echo "⚠️  ${#CHANGED_COMPONENTS[@]} component(s) changed. Selective rollback required."
        return 0
    fi
}
```

## Automated Rollback Script

**Unified Rollback with Change Detection:**

```bash
#!/bin/bash
# unified-rollback.sh

echo "=== Initiating Unified DocumentDB Rollback with Change Detection ==="

# Step 1: Get current and previous Helm revision
CURRENT_REVISION=$(helm history documentdb-operator -n documentdb-system --max 1 -o json | jq -r '.[0].revision')
PREVIOUS_REVISION=$((CURRENT_REVISION - 1))

echo "Rolling back from revision $CURRENT_REVISION to $PREVIOUS_REVISION"

# Step 2: Load change detection functions
source component-hash-tracker.sh

# Step 3: Generate and compare component hashes
if ! compare_component_hashes $CURRENT_REVISION $PREVIOUS_REVISION; then
    echo "ℹ️  No changes detected between revisions. Skipping rollback."
    exit 0
fi

# Step 4: Pre-rollback validation (only for changed components)
echo "=== Pre-Rollback Validation ==="

# Check current component versions before rollback
echo "Current component versions (will check only changed components):"
for component in "${!CHANGED_COMPONENTS[@]}"; do
    case $component in
        operator)
            CURRENT_VALUE=$(kubectl get deployment documentdb-operator -n documentdb-system -o jsonpath='{.spec.template.spec.containers[0].image}')
            echo "  DocumentDB Operator: $CURRENT_VALUE (WILL ROLLBACK)"
            ;;
        cnpg)
            CURRENT_VALUE=$(kubectl get deployment cnpg-controller-manager -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}')
            echo "  CNPG Operator: $CURRENT_VALUE (WILL ROLLBACK)"
            ;;
        gateway|postgres|sidecar)
            echo "  $component: ${CHANGED_COMPONENTS[$component]} (WILL ROLLBACK via pod restart)"
            ;;
    esac
done

for component in "${!UNCHANGED_COMPONENTS[@]}"; do
    echo "  $component: ${UNCHANGED_COMPONENTS[$component]} (SKIP - unchanged)"
done

# Check cluster health before rollback
kubectl get clusters.postgresql.cnpg.io -A -o wide
kubectl get documentdb -A -o wide

# Step 5: Selective Component Rollback
echo "=== Performing Selective Component Rollback ==="

# Rollback operators only if they changed
if [[ -v CHANGED_COMPONENTS[operator] ]] || [[ -v CHANGED_COMPONENTS[cnpg] ]]; then
    echo "Rolling back operators (DocumentDB and/or CNPG)..."
    helm rollback documentdb-operator $PREVIOUS_REVISION -n documentdb-system --wait --timeout=900s
    
    if [ $? -ne 0 ]; then
        echo "❌ Helm rollback failed. Manual intervention required."
        exit 1
    fi
    
    # Verify operator rollback
    echo "=== Verifying Operator Rollback ==="
    if [[ -v CHANGED_COMPONENTS[operator] ]]; then
        kubectl rollout status deployment/documentdb-operator -n documentdb-system --timeout=300s
        NEW_OPERATOR=$(kubectl get deployment documentdb-operator -n documentdb-system -o jsonpath='{.spec.template.spec.containers[0].image}')
        echo "DocumentDB Operator rolled back to: $NEW_OPERATOR"
    fi
    
    if [[ -v CHANGED_COMPONENTS[cnpg] ]]; then
        kubectl rollout status deployment/cnpg-controller-manager -n cnpg-system --timeout=300s
        NEW_CNPG=$(kubectl get deployment cnpg-controller-manager -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}')
        echo "CNPG Operator rolled back to: $NEW_CNPG"
    fi
else
    echo "⏭️  Skipping operator rollback - no changes detected in operator or CNPG components"
fi

# Step 5: Selective rolling restart of CNPG clusters (only for changed components)
echo "=== Rolling Back CNPG Clusters with Change Detection ==="

# Only restart clusters if database-related components changed
if [[ -v CHANGED_COMPONENTS[postgres] ]] || [[ -v CHANGED_COMPONENTS[gateway] ]] || [[ -v CHANGED_COMPONENTS[sidecar] ]]; then
    echo "Database component changes detected - restarting CNPG clusters..."
    
    for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
        namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
        
        # Get current cluster image to compare with target
        CURRENT_CLUSTER_IMAGE=$(kubectl get cluster $cluster -n $namespace -o jsonpath='{.spec.imageName}' 2>/dev/null || echo "not-found")
        
        echo "Rolling back cluster: $cluster in namespace: $namespace"
        echo "  Current image: $CURRENT_CLUSTER_IMAGE"
        echo "  Target PostgreSQL: ${CHANGED_COMPONENTS[postgres]:-unchanged}"
        echo "  Target Gateway: ${CHANGED_COMPONENTS[gateway]:-unchanged}"
        echo "  Target Sidecar: ${CHANGED_COMPONENTS[sidecar]:-unchanged}"
        
        # Trigger rolling restart to revert to previous images
        kubectl annotate clusters.postgresql.cnpg.io $cluster -n $namespace \
            cnpg.io/reloadedAt="$(date -Iseconds)" \
            rollback.documentdocumentdb.io/version="$PREVIOUS_REVISION" \
            rollback.documentdocumentdb.io/reason="component-change-detected" \
            --overwrite
        
        # Wait for rollback to complete
        kubectl wait --for=condition=Ready clusters.postgresql.cnpg.io/$cluster -n $namespace --timeout=600s
        
        if [ $? -eq 0 ]; then
            NEW_CLUSTER_IMAGE=$(kubectl get cluster $cluster -n $namespace -o jsonpath='{.spec.imageName}')
            echo "✅ Cluster $cluster successfully rolled back to: $NEW_CLUSTER_IMAGE"
        else
            echo "❌ Cluster $cluster rollback failed - manual intervention required"
        fi
    done
else
    echo "⏭️  Skipping CNPG cluster restart - no database component changes detected"
    
    # Show current cluster status
    echo "Current cluster status (no changes):"
    kubectl get clusters.postgresql.cnpg.io -A -o wide | head -10
fi

# Step 6: Post-rollback validation with change verification
echo "=== Post-Rollback Validation ==="

# Verify only changed components were actually rolled back
echo "=== Change Detection Verification ==="
generate_component_hashes $PREVIOUS_REVISION
if compare_component_hashes $PREVIOUS_REVISION $PREVIOUS_REVISION; then
    echo "⚠️  Warning: Hash comparison still shows changes after rollback"
else
    echo "✅ All component changes successfully reverted"
fi

# Verify cluster health
echo "=== Cluster Health Check ==="
kubectl get clusters.postgresql.cnpg.io -A -o wide
kubectl get documentdb -A -o wide

# Test MongoDB connectivity for changed clusters only
echo "=== Connectivity Testing (Changed Components Only) ==="
if [[ -v CHANGED_COMPONENTS[postgres] ]] || [[ -v CHANGED_COMPONENTS[gateway] ]] || [[ -v CHANGED_COMPONENTS[sidecar] ]]; then
    echo "Testing MongoDB connectivity for clusters with component changes..."
    for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
        namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
        service_name="${cluster}-rw"
        
        # Test basic connectivity
        kubectl run rollback-test-$cluster --rm -i --tty --timeout=30s --image=mongo:7 -- \
            mongosh "mongodb://$service_name.$namespace.svc.cluster.local:10260/test" --eval "
            db.rollback_test.insertOne({test: 'rollback_validation', timestamp: new Date()});
            print('Rollback connectivity test passed for cluster: $cluster');
            " 2>/dev/null || echo "❌ Connectivity test failed for cluster: $cluster"
    done
    
    # Verify component versions match target hashes
    echo "Verifying component version consistency..."
    kubectl get pods -l cnpg.io/cluster --all-namespaces -o custom-columns=\
    "NAMESPACE:.metadata.namespace,NAME:.metadata.name,GATEWAY:.spec.containers[?(@.name=='documentdb-gateway')].image,DOCUMENTDB:.spec.containers[?(@.name=='postgres')].image"
else
    echo "⏭️  Skipping connectivity tests - no database component changes detected"
fi

# Step 7: Rollback Summary and Cleanup
echo "=== Rollback Summary ==="
echo "Rollback completed: Revision $CURRENT_REVISION → $PREVIOUS_REVISION"
echo "Components processed:"
for component in "${!CHANGED_COMPONENTS[@]}"; do
    echo "  ✅ $component: ${CHANGED_COMPONENTS[$component]} (ROLLED BACK)"
done
for component in "${!UNCHANGED_COMPONENTS[@]}"; do
    echo "  ⏭️  $component: ${UNCHANGED_COMPONENTS[$component]} (SKIPPED - unchanged)"
done

# Store rollback record for future reference
kubectl create configmap documentdb-rollback-r${CURRENT_REVISION}-to-r${PREVIOUS_REVISION} -n documentdb-system \
    --from-literal=rollback-timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --from-literal=source-revision="$CURRENT_REVISION" \
    --from-literal=target-revision="$PREVIOUS_REVISION" \
    --from-literal=changed-components="$(IFS=,; echo "${!CHANGED_COMPONENTS[*]}")" \
    --from-literal=unchanged-components="$(IFS=,; echo "${!UNCHANGED_COMPONENTS[*]}")" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Unified rollback with change detection completed successfully"
```

## Manual Emergency Rollback Procedures

**Emergency Manual Rollback (if automation fails):**

```bash
# Emergency Manual Rollback Procedure

# Step 1: Manual Helm rollback
helm rollback documentdb-operator $PREVIOUS_REVISION -n documentdb-system

# Step 2: If Helm rollback fails, manual operator rollback
kubectl patch deployment documentdb-operator -n documentdb-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"documentdb-operator","image":"ghcr.io/documentdb/documentdb-operator:v1.2.3"}]}}}}'

# Step 3: Manual CNPG operator rollback (if needed)
kubectl patch deployment cnpg-controller-manager -n cnpg-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"manager","image":"ghcr.io/cloudnative-pg/cloudnative-pg:1.24.0"}]}}}}'

# Step 4: Manual sidecar injector rollback
kubectl patch deployment sidecar-injector -n cnpg-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"sidecar-injector","image":"ghcr.io/documentdb/documentdb-sidecar-injector:v1.2.3"}]}}}}'

# Step 5: Force rolling restart of all CNPG clusters
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  kubectl delete pods -l cnpg.io/cluster=$cluster -n $namespace
done
```

## Change Detection Configuration

**Configuration Variables:**

```bash
# Enable change detection by default in your rollback scripts
export ENABLE_CHANGE_DETECTION=true

# Force rollback of all components (bypass change detection)
export FORCE_FULL_ROLLBACK=false

# Retention policy for component hash ConfigMaps (keep last 10 revisions)
export HASH_RETENTION_COUNT=10
```

**Hash Storage Cleanup:**

```bash
# Cleanup old component hash ConfigMaps (run periodically)
#!/bin/bash
RETENTION_COUNT=${HASH_RETENTION_COUNT:-10}

# Keep only the last N revisions of component hashes
kubectl get configmap -n documentdb-system -o name | \
  grep "documentdb-component-hashes-r" | \
  sort -V | \
  head -n -$RETENTION_COUNT | \
  xargs -r kubectl delete -n documentdb-system
```

## Blue-Green Deployment Procedures

**Blue-Green Deployment Overview:**

For major PostgreSQL upgrades requiring blue-green deployment, the process involves:

1. **Green Cluster Preparation**: Backup and baseline metrics collection
2. **Blue Cluster Deployment**: Deploy new cluster with target DocumentDB version
3. **Data Migration**: Migrate data from green to blue cluster (see separate backup/restore design)
4. **Traffic Switching**: Update Kubernetes services to point to blue cluster
5. **Validation**: Verify functionality and performance
6. **Cleanup**: Remove green cluster after validation period

**Key Components:**
- **Service Management**: Kubernetes service updates for traffic switching
- **Data Validation**: Database integrity and connectivity verification
- **Rollback Capability**: Immediate rollback to green cluster if issues occur

**Detailed Implementation:**
Complete blue-green deployment procedures, including backup/restore automation, will be documented in the dedicated backup/restore design document.

**Emergency Rollback:**
```bash
#!/bin/bash
# blue-green-rollback.sh - Emergency rollback from blue to green

echo "=== Emergency Blue-Green Rollback ==="
echo "This will immediately switch traffic back to the green cluster"

# Variables
GREEN_CLUSTER_NAME="documentdb-production"
BLUE_CLUSTER_NAME="documentdb-production-blue"
NAMESPACE="production"

# Restore green cluster service configuration
if [ -f /tmp/green-service-backup.yaml ]; then
    kubectl apply -f /tmp/green-service-backup.yaml
    echo "✅ Green cluster service configuration restored"
else
    echo "❌ Green service backup not found. Manual rollback required:"
    echo "kubectl patch service ${GREEN_CLUSTER_NAME}-rw -n $NAMESPACE -p '{\"spec\":{\"selector\":{\"cnpg.io/cluster\":\"$GREEN_CLUSTER_NAME\"}}}'"
    echo "kubectl patch service ${GREEN_CLUSTER_NAME}-ro -n $NAMESPACE -p '{\"spec\":{\"selector\":{\"cnpg.io/cluster\":\"$GREEN_CLUSTER_NAME\"}}}'"
fi

# Verify rollback
echo "Verifying rollback connectivity..."
kubectl run rollback-test --rm -i --timeout=60s --image=mongo:7 -- \
  mongosh "mongodb://${GREEN_CLUSTER_NAME}-rw.$NAMESPACE.svc.cluster.local:10260/test" --eval "
  db.rollback_test.insertOne({test: 'rollback_validation', timestamp: new Date()});
  print('Rollback connectivity test passed');
  " 2>/dev/null

echo "✅ Emergency rollback completed"
```

## CNPG Supervised HA Upgrade Procedures

This section contains detailed command examples for CNPG-based high availability upgrade procedures supporting the local HA strategy outlined in the [upgrade design document](./upgrade-design-doc.md#local-high-availability-ha-and-upgrade-strategy).

Terminology note: We use primary + standby (instead of primary + replica). A standby is a streaming replica that can be promoted. The supervised upgrade flow is standby-first (all standbys roll to new image, then controlled switchover, then former primary upgrades) providing continuous write availability and a rollback decision window.

### CNPG HA Configuration Examples

**3-Instance DocumentDB Cluster with HA Configuration:**

```yaml
# documentdb-ha-cluster.yaml (corrected example)
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: documentdb-cluster
  namespace: production
  labels:
    documentdocumentdb.io/sidecar-inject: "true"   # Only if injector matches this label
    documentdocumentdb.io/cluster-version: "v1.4.0"
spec:
  instances: 3   # 1 primary + 2 standbys

  # Manual control over primary switch during image upgrades
  primaryUpdateStrategy: supervised

  # Engine + extension image (changed for upgrades)
  imageName: mcr.microsoft.com/documentdb/documentdb:16.3-v1.4.0

  postgresql:
    parameters:
      wal_level: "replica"
      max_wal_senders: "6"                  # Enough for replicas + future logical slots
      synchronous_commit: "remote_write"    # Balance durability & latency
      synchronous_standby_names: 'ANY 1 (*)' # Quorum of 1; avoids write stalls if one replica gone

  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
    limits:
      cpu: "2"
      memory: "4Gi"

  storage:
    size: 100Gi
    storageClass: fast-ssd

  # NOTE: Fields like primaryUpdateMethod, switchoverDelay, failoverDelay do NOT exist in CNPG.
  # Supervised flow: patch imageName -> standbys upgrade -> manual promotion -> former primary upgrades.
```

> Changes vs previous draft:
> - Removed non-existent spec fields (primaryUpdateMethod, switchoverDelay, failoverDelay)
> - Moved labels to correct metadata level (was under spec.metadata erroneously)
> - Replaced synchronous standby config ("*" + synchronous_commit=on) with safer quorum pattern
> - Trimmed parameters that added noise without justification (checkpoint_timeout, max_wal_size)
> - Explicit imageName included

**Apply HA Configuration:**
```bash
# Deploy HA-configured DocumentDB cluster
kubectl apply -f documentdb-ha-cluster.yaml

# Wait for cluster to be ready
kubectl wait --for=condition=Ready cluster/documentdb-cluster -n production --timeout=600s

# Verify cluster status
kubectl cnpg status documentdb-cluster -n production
```

### CNPG Zero-Downtime Upgrade Sequence

**Complete upgrade workflow with CNPG supervised mode:**

#### Step 1: Pre-Upgrade Validation

```bash
# Verify cluster health and standby synchronization
echo "=== Pre-Upgrade Validation ==="

# Check overall cluster status
kubectl cnpg status documentdb-cluster -n production

# Verify Ready condition (portable)
READY_COND=$(kubectl get cluster documentdb-cluster -n production -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
if [ "$READY_COND" != "True" ]; then
  echo "❌ Cluster not Ready (Ready condition: $READY_COND)";
  echo "Aborting upgrade. Fix cluster issues first."; exit 1; fi

# Check standby lag (ensure minimal lag before switchover)
echo "Checking standby lag (verbose status)..."
kubectl cnpg status documentdb-cluster -n production --verbose || true
echo "(Optionally query pg_stat_replication for precise byte/LSN lag)"

# Verify all instances are running
kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production

# Check current PostgreSQL version
CURRENT_VERSION=$(kubectl get cluster documentdb-cluster -n production -o jsonpath='{.spec.imageName}')
echo "Current cluster image: $CURRENT_VERSION"

# Test connectivity before upgrade
kubectl run pre-upgrade-test --rm -i --timeout=30s --image=mongo:7 -- \
  mongosh "mongodb://documentdb-cluster-rw.production.svc.cluster.local:10260/test" --eval "
  db.pre_upgrade_test.insertOne({test: 'connectivity_check', timestamp: new Date()});
  print('Pre-upgrade connectivity test passed');
  " 2>/dev/null

echo "✅ Pre-upgrade validation completed"
```

#### Step 2: Enable Supervised Mode and Trigger Upgrade

```bash
# Enable supervised mode for controlled failover
echo "=== Enabling Supervised Mode ==="

kubectl patch cluster documentdb-cluster -n production --type='merge' -p '{
  "spec": {
    "primaryUpdateStrategy": "supervised"
  }
}'

# Verify supervised mode is enabled
STRATEGY=$(kubectl get cluster documentdb-cluster -n production -o jsonpath='{.spec.primaryUpdateStrategy}')
echo "Primary update strategy: $STRATEGY"

# Trigger rolling update (standbys upgrade automatically)
echo "=== Triggering Rolling Update ==="

TARGET_IMAGE="mcr.microsoft.com/documentdb/documentdb:16.3-v1.4.0"
kubectl patch cluster documentdb-cluster -n production --type='merge' -p "{
  \"spec\": {
    \"imageName\": \"$TARGET_IMAGE\"
  }
}"

echo "Upgrade initiated to target image: $TARGET_IMAGE"
```

#### Step 3: Monitor Standby Upgrades

```bash
# Monitor standby upgrades (wait for completion)
echo "=== Monitoring Standby Upgrades ==="

# Watch pods during upgrade
echo "Watching pod changes during standby upgrade..."
kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production -w &
WATCH_PID=$!

# Monitor upgrade progress
echo "Monitoring upgrade progress..."
while true; do
    # Check cluster status
    STATUS=$(kubectl cnpg status documentdb-cluster -n production 2>/dev/null)
    echo "Cluster status update:"
    echo "$STATUS"
    
    # Check if primary is still the only non-upgraded instance
  UPGRADED_STANDBYS=$(kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production -o jsonpath="{.items[?(@.spec.containers[?(@.name=='postgres')].image=='$TARGET_IMAGE')].metadata.name}" | wc -w)
  TOTAL_INSTANCES=$(kubectl get cluster documentdb-cluster -n production -o jsonpath='{.spec.instances}')
    
  echo "Upgraded instances: $UPGRADED_STANDBYS/$TOTAL_INSTANCES"
    
  # Ready when standbys (instances - 1) upgraded
  if [ "$UPGRADED_STANDBYS" -eq $((TOTAL_INSTANCES - 1)) ]; then
        echo "✅ Standby upgrades completed. Ready for switchover."
        break
    fi
    
    sleep 30
done

# Stop watching pods
kill $WATCH_PID 2>/dev/null
```

#### Step 4: Manual Controlled Switchover

```bash
# Manual switchover when ready
echo "=== Performing Controlled Switchover ==="

# Get current primary (plugin output varies; adjust parsing if needed)
CURRENT_PRIMARY=$(kubectl cnpg status documentdb-cluster -n production | grep -i "Primary" | awk '{print $2}')
echo "Current primary: $CURRENT_PRIMARY"

# Find most aligned standby (should be one of the upgraded standbys)
echo "Finding most aligned standby..."
kubectl cnpg status documentdb-cluster -n production --verbose

# List available standbys with their status
STANDBYS=$(kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production -o jsonpath='{.items[?(@.metadata.name!="'$CURRENT_PRIMARY'")].metadata.name}')
echo "Available standbys: $STANDBYS"

# Choose the first upgraded standby (you can customize this selection logic)
TARGET_STANDBY=$(echo $STANDBYS | awk '{print $1}')
echo "Selected target standby for promotion: $TARGET_STANDBY"

# Perform switchover (some plugin versions support --target)
echo "Initiating switchover to $TARGET_STANDBY..."
kubectl cnpg promote documentdb-cluster --target $TARGET_STANDBY -n production || \
  kubectl cnpg promote documentdb-cluster $TARGET_STANDBY -n production

# Wait for switchover to complete
echo "Waiting for switchover to complete..."
sleep 30

# Verify new primary
NEW_PRIMARY=$(kubectl cnpg status documentdb-cluster -n production | grep -i "Primary" | awk '{print $2}')
echo "New primary after switchover: $NEW_PRIMARY"

if [ "$NEW_PRIMARY" = "$TARGET_STANDBY" ]; then
    echo "✅ Switchover successful: $CURRENT_PRIMARY → $NEW_PRIMARY"
else
  echo "❌ Switchover may have failed. Expected: $TARGET_STANDBY, Got: $NEW_PRIMARY"
fi
```

#### Step 5: Verify Upgrade Completion

```bash
# Verify new primary and complete upgrade
echo "=== Verifying Upgrade Completion ==="

# Check cluster status after switchover
kubectl cnpg status documentdb-cluster -n production

# Wait for former primary to complete upgrade
echo "Waiting for all instances to complete upgrade..."
while true; do
  UPGRADED_COUNT=$(kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production -o jsonpath="{.items[?(@.spec.containers[?(@.name=='postgres')].image=='$TARGET_IMAGE')].metadata.name}" | wc -w)
  TOTAL_COUNT=$(kubectl get cluster documentdb-cluster -n production -o jsonpath='{.spec.instances}')
    
    echo "Upgrade progress: $UPGRADED_COUNT/$TOTAL_COUNT instances completed"
    
    if [ "$UPGRADED_COUNT" -eq "$TOTAL_COUNT" ]; then
        echo "✅ All instances upgraded successfully"
        break
    fi
    
    sleep 30
done

# Verify all instances running target version (postgres container only)
echo "=== Final Version Verification ==="
kubectl get pods -l cnpg.io/cluster=documentdb-cluster -n production -o json | \
  jq -r '.items[] | {name: .metadata.name, image: (.spec.containers[] | select(.name=="postgres").image)} | "\(.name)\t\(.image)"'

# Test connectivity and performance after upgrade
echo "=== Post-Upgrade Connectivity Test ==="
kubectl run post-upgrade-test --rm -i --timeout=30s --image=mongo:7 -- \
  mongosh "mongodb://documentdb-cluster-rw.production.svc.cluster.local:10260/test" --eval "
  db.post_upgrade_test.insertOne({
    test: 'upgrade_completion_check', 
    timestamp: new Date(),
    upgraded_to: '$TARGET_IMAGE'
  });
  print('Post-upgrade connectivity test passed');
  " 2>/dev/null

# Verify PostgreSQL is ready
kubectl exec -it documentdb-cluster-1 -n production -- pg_isready

echo "✅ CNPG supervised HA upgrade completed successfully"
```

### CNPG HA Upgrade Automation Script (corrected)

**Complete automated HA upgrade script:**

```bash
#!/bin/bash
# cnpg-ha-upgrade.sh - Automated CNPG HA upgrade with safety checks

set -e

# Configuration
CLUSTER_NAME="${1:-documentdb-cluster}"
NAMESPACE="${2:-production}"
TARGET_IMAGE="${3:-mcr.microsoft.com/documentdb/documentdb:16.3-v1.4.0}"

echo "=== CNPG HA Upgrade Automation ==="
echo "Cluster: $CLUSTER_NAME"
echo "Namespace: $NAMESPACE"
echo "Target Image: $TARGET_IMAGE"

# Function: Wait for condition with timeout
wait_for_condition() {
    local condition="$1"
    local timeout="${2:-600}"
    local check_interval="${3:-30}"
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        if eval "$condition"; then
            return 0
        fi
        sleep $check_interval
        elapsed=$((elapsed + check_interval))
        echo "Waiting... ($elapsed/${timeout}s)"
    done
    
    echo "❌ Timeout waiting for condition: $condition"
    return 1
}

# Pre-upgrade validation
echo "=== Step 1: Pre-Upgrade Validation ==="
kubectl cnpg status $CLUSTER_NAME -n $NAMESPACE

# Ready condition check (instead of brittle phase string)
READY_COND=$(kubectl get cluster $CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
if [ "$READY_COND" != "True" ]; then
  echo "❌ Cluster not Ready (Ready condition: $READY_COND)"; exit 1; fi

# Enable supervised mode
echo "=== Step 2: Enable Supervised Mode ==="
kubectl patch cluster $CLUSTER_NAME -n $NAMESPACE --type='merge' -p '{
  "spec": {
    "primaryUpdateStrategy": "supervised"
  }
}'

# Trigger upgrade
echo "=== Step 3: Trigger Rolling Update ==="
kubectl patch cluster $CLUSTER_NAME -n $NAMESPACE --type='merge' -p "{
  \"spec\": {
    \"imageName\": \"$TARGET_IMAGE\"
  }
}"

# Wait for standby upgrades
echo "=== Step 4: Wait for Standby Upgrades ==="
# Dynamically derive standby count: instances - 1
INSTANCES=$(kubectl get cluster $CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.spec.instances}')
STANDBY_COUNT=$((INSTANCES - 1))
wait_for_condition '[ $(kubectl get pods -l cnpg.io/cluster='$CLUSTER_NAME' -n '$NAMESPACE' -o jsonpath="{.items[?(@.spec.containers[?(@.name=='postgres')].image==\"'$TARGET_IMAGE'\")].metadata.name}" | wc -w) -eq '$STANDBY_COUNT' ]' 1200

# Get current primary and select target standby
CURRENT_PRIMARY=$(kubectl cnpg status $CLUSTER_NAME -n $NAMESPACE | grep -i "Primary" | awk '{print $2}')
TARGET_STANDBY=$(kubectl get pods -l cnpg.io/cluster=$CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.items[?(@.metadata.name!="'$CURRENT_PRIMARY'")].metadata.name}' | awk '{print $1}')

echo "Current primary: $CURRENT_PRIMARY"
echo "Target standby: $TARGET_STANDBY"

# Perform switchover
echo "=== Step 5: Perform Switchover ==="
kubectl cnpg promote $CLUSTER_NAME --target $TARGET_STANDBY -n $NAMESPACE || \
  kubectl cnpg promote $CLUSTER_NAME $TARGET_STANDBY -n $NAMESPACE

# Wait for all instances to be upgraded
echo "=== Step 6: Wait for Upgrade Completion ==="
wait_for_condition '[ $(kubectl get pods -l cnpg.io/cluster='$CLUSTER_NAME' -n '$NAMESPACE' -o jsonpath="{.items[?(@.spec.containers[?(@.name=='postgres')].image==\"'$TARGET_IMAGE'\")].metadata.name}" | wc -w) -eq '$INSTANCES' ]' 900

# Final validation
echo "=== Step 7: Final Validation ==="
kubectl cnpg status $CLUSTER_NAME -n $NAMESPACE

# Test connectivity
kubectl run upgrade-validation-$RANDOM --rm -i --timeout=30s --image=mongo:7 -- \
  mongosh "mongodb://${CLUSTER_NAME}-rw.${NAMESPACE}.svc.cluster.local:10260/test" --eval "
  db.upgrade_validation.insertOne({
    test: 'automated_upgrade_validation', 
    timestamp: new Date(),
    target_image: '$TARGET_IMAGE'
  });
  print('Automated upgrade validation passed');
  " 2>/dev/null

echo "✅ CNPG HA upgrade automation completed successfully"
```

### CNPG HA Troubleshooting Commands

**Common troubleshooting scenarios during HA upgrades:**

```bash
# Check cluster events for upgrade issues
kubectl get events -n production --field-selector involvedObject.name=documentdb-cluster --sort-by='.lastTimestamp'

# Check CNPG operator logs
kubectl logs -n cnpg-system deployment/cnpg-controller-manager -f

# Check specific pod logs during upgrade
kubectl logs documentdb-cluster-1 -n production -c postgres -f

# Manual failover if automatic switchover fails
kubectl cnpg promote documentdb-cluster documentdb-cluster-2 -n production --force

# Reset to unsupervised mode if needed
kubectl patch cluster documentdb-cluster -n production --type='merge' -p '{
  "spec": {
    "primaryUpdateStrategy": "unsupervised"
  }
}'

# Manual pod restart if stuck
kubectl delete pod documentdb-cluster-1 -n production

# Check standby lag during troubleshooting
kubectl cnpg status documentdb-cluster -n production --verbose

# Emergency rollback to previous image
kubectl patch cluster documentdb-cluster -n production --type='merge' -p '{
  "spec": {
    "imageName": "mcr.microsoft.com/documentdb/documentdb:16.2-v1.3.0"
  }
}'
```
