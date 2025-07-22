# DocumentDB Kubernetes Operator Upgrade Design

## Overview

This document outlines the upgrade strategy for the DocumentDB Kubernetes operator, which provides a MongoDB-compatible API layer over PostgreSQL using the CloudNative-PG (CNPG) operator. The system consists of multiple components that require coordinated upgrades to ensure service continuity and data integrity.

## Required Knowledge

Before implementing the upgrade strategies outlined in this document, readers should have a solid understanding of:

### 1. Kubernetes Operators
- **Operator Pattern**: Understanding of Custom Resource Definitions (CRDs), Controllers, and reconciliation loops
- **Operator Lifecycle Management**: How operators manage application state and handle updates
- **Webhook Management**: Admission controllers and certificate management

### 2. DocumentDB Operator Architecture
- **System Components**: DocumentDB Operator, Gateway containers, PostgreSQL with extensions, and sidecar injection
- **CNPG Integration**: How DocumentDB operator leverages CloudNative-PG for PostgreSQL cluster management
- **Resource Relationships**: Understanding the dependency chain between components

### 3. Helm Chart Management
- **Chart Dependencies**: Managing upstream dependencies and version compatibility
- **CRD Handling**: Helm limitations with CRD upgrades and migration strategies
- **Rollback Procedures**: Helm rollback capabilities and limitations

### 4. PostgreSQL and Extensions
- **Version Compatibility**: Major vs minor version implications and extension dependencies
- **Backup and Recovery**: PostgreSQL backup strategies and point-in-time recovery
- **Data Migration**: Understanding of schema migration and data integrity validation

This foundational knowledge ensures that operators implementing these upgrade strategies understand the underlying architecture and can make informed decisions during the upgrade process.

## Architecture Components

**Important**: DocumentDB uses a **unified versioning strategy** where all components are versioned together for simplicity and compatibility assurance.

### Unified Versioning Strategy

**Single Version for All Components:**
- **DocumentDB Operator Version**: `v1.2.3` controls all component versions
- **Gateway Image**: Automatically aligned (e.g., `ghcr.io/microsoft/documentdb/gateway:v1.2.3`)
- **PostgreSQL + Extension**: Automatically aligned (e.g., `mcr.microsoft.com/documentdb/documentdb:16.2-v1.2.3`)
- **Sidecar Injector**: Automatically aligned (e.g., `ghcr.io/microsoft/documentdb/sidecar-injector:v1.2.3`)
- **CNPG Operator**: Locked to DocumentDB version (e.g., DocumentDB v1.2.3 → CNPG v0.24.0)

**Benefits of Unified Versioning:**
- **Simplified Operations**: One version to track instead of managing multiple component versions
- **Guaranteed Compatibility**: All components tested together as a cohesive unit
- **Reduced Complexity**: Eliminates version matrix compatibility issues
- **Easier Rollbacks**: Single version rollback affects all components consistently
- **Clear Release Management**: Single release pipeline for all components

### Component Details

### 1. DocumentDB Operator (Master Version)
- **Component**: Custom Kubernetes operator (Go-based)
- **Distribution**: Helm chart
- **Function**: Manages DocumentDB custom resources and orchestrates CNPG clusters
- **Versioning**: **Master version** (e.g., v1.2.3) - drives all other component versions

### 2. Gateway Container (Aligned)
- **Component**: DocumentDB Gateway (MongoDB protocol translator)
- **Distribution**: Docker image
- **Function**: Translates MongoDB protocol to PostgreSQL queries
- **Versioning**: **Automatically aligned** with operator version (e.g., gateway:v1.2.3)

### 3. PostgreSQL with DocumentDB Extension (Aligned)
- **Component**: PostgreSQL server with DocumentDB extensions
- **Distribution**: Docker image
- **Function**: Data storage and processing layer
- **Versioning**: **Automatically aligned** with operator version (e.g., postgres:16.2-v1.2.3)

### 4. CNPG Sidecar Injector (Aligned)
- **Component**: Sidecar injection webhook
- **Distribution**: Docker image
- **Function**: Injects Gateway sidecar into CNPG pods
- **Versioning**: **Automatically aligned** with operator version (e.g., sidecar-injector:v1.2.3)

### 5. CNPG Operator (Locked Dependency)
- **Component**: CloudNative-PG operator
- **Distribution**: Helm chart (as dependency)
- **Function**: PostgreSQL cluster management
- **Versioning**: **Locked to DocumentDB operator version** (e.g., DocumentDB v1.2.3 → CNPG v0.24.0)

## Upgrade Scenarios

**With Unified Versioning**: All component upgrades are triggered by a single DocumentDB operator version upgrade. Individual component upgrades are not available to customers.

### 1. DocumentDB Operator Upgrade (Unified Release)
- **Trigger**: New DocumentDB operator version release (e.g., v1.2.3 → v1.3.0)
- **Scope**: **All components** upgraded atomically
- **Impact**: All components (operator, gateway, postgres, sidecar injector) upgraded together
- **Components Included**:
  - DocumentDB Operator: v1.2.3 → v1.3.0
  - Gateway Image: v1.2.3 → v1.3.0
  - PostgreSQL + Extension: 16.2-v1.2.3 → 16.3-v1.3.0 (or 17.1-v1.3.0 for major PG upgrades)
  - Sidecar Injector: v1.2.3 → v1.3.0
  - CNPG Operator: v0.24.0 → v0.26.0 (if required)

### 2. Component-Specific Upgrade Considerations

While all components upgrade together, each has specific characteristics:

#### Gateway Image Upgrade (Part of Unified Release)
- **State**: **Stateless** - Gateway containers have no persistent state
- **Impact**: Medium (rolling restart of pods)
- **Risk**: Low - No data loss risk, only temporary connection disruption

#### PostgreSQL Database Upgrade (Part of Unified Release)  
- **State**: **Stateful** - PostgreSQL contains persistent application data
- **Impact**: High (potential data migration and downtime required)
- **Risk**: High - Data migration required, potential for data corruption or loss
- **Categories**:
  - **Minor Version**: 14.2-v1.2.3 → 14.3-v1.3.0 (in-place upgrade, low risk)
  - **Major Version**: 14.2-v1.2.3 → 15.1-v1.3.0 (migration required, high risk)

#### DocumentDB Postgres Extension Upgrade (Part of Unified Release)
- **State**: **Stateful** - Extension may modify schema or data structures  
- **Impact**: Medium to High (depends on extension changes)
- **Risk**: Medium to High - Extension schema changes may affect data
- **Categories**:
  - **Patch Updates**: Bug fixes, minor improvements (medium risk)
  - **Feature Updates**: New DocumentDB features, API changes (high risk)
  - **Breaking Changes**: Schema modifications, compatibility breaks (very high risk)

#### Sidecar Injector Upgrade (Part of Unified Release)
- **State**: **Stateless** - Injection webhook has no persistent state
- **Impact**: Medium (affects new pod creation)
- **Risk**: Medium - Injection failures affect new PostgreSQL pods

#### CNPG Operator Upgrade (Part of Unified Release)
- **Trigger**: Upgrade bundled with DocumentDB operator when CNPG version needs updating
- **Scope**: Control plane and data plane
- **Impact**: Variable (depends on CNPG upgrade requirements)

## Upgrade Strategies

**With Unified Versioning**: There is only **one primary upgrade strategy** - upgrading the DocumentDB operator version, which automatically upgrades all components.

### 1. Unified DocumentDB Upgrade Strategy

The DocumentDB upgrade process involves upgrading all components together through a single DocumentDB operator version upgrade. This ensures consistency, compatibility, and simplified operations.

#### A. Unified Components Upgrade

**All Components Upgraded Together:**
- **DocumentDB Operator**: Controller, webhooks, CRDs, RBAC
- **Gateway Image**: MongoDB protocol translator containers
- **PostgreSQL + Extension**: Database with DocumentDB extension
- **Sidecar Injector**: Container injection webhook
- **CNPG Operator**: PostgreSQL cluster management (when version updates required)

**Version Alignment Example:**
```yaml
# DocumentDB v1.3.0 Release Bundle
documentdb-operator: v1.3.0
gateway: v1.3.0  
postgres: 16.3-v1.3.0  # PostgreSQL 16.3 + DocumentDB extension v1.3.0
sidecar-injector: v1.3.0
cnpg-operator: v0.26.0  # Updated if required for this release
```

#### B. Unified Upgrade Strategy

**Single Helm Upgrade Command (All Components):**

```bash
# Step 1: Pre-upgrade validation
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --version v1.3.0 \
  --dry-run \
  --debug

# Step 2: Perform unified upgrade (all components)
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --version v1.3.0 \
  --wait \
  --timeout 900s \
  --atomic

# Step 3: Verify all components health
kubectl rollout status deployment/documentdb-operator -n documentdb-system
kubectl rollout status deployment/sidecar-injector -n cnpg-system
kubectl get clusters.postgresql.cnpg.io -A -o wide
```

**Upgrade Process Flow:**
1. **CNPG Operator** (if version update required)
2. **DocumentDB Operator** (CRDs, controller, webhooks, RBAC)
3. **Sidecar Injector** (webhook for container injection)
4. **New pods created** with updated Gateway and PostgreSQL images
5. **Rolling restart** of existing CNPG clusters to get new images

#### C. Component-Specific Handling Within Unified Upgrade

While all components upgrade together, each requires specific handling:

##### PostgreSQL Upgrade Handling
**For Minor PostgreSQL Versions** (e.g., 16.2-v1.2.3 → 16.3-v1.3.0):
```yaml
# CNPG handles minor PostgreSQL upgrades automatically  
spec:
  imageName: "mcr.microsoft.com/documentdb/documentdb:16.3-v1.3.0"
```

**For Major PostgreSQL Versions** (e.g., 16.2-v1.2.3 → 17.1-v1.3.0):
- Blue-green deployment recommended for major PostgreSQL upgrades
- Data migration required
- Extended maintenance window needed

##### Gateway Image Handling
- **Stateless**: No data migration required
- **Rolling restart**: CNPG automatically restarts pods with new gateway image
- **Zero data loss**: Only temporary connection interruption

##### Extension Upgrade Handling
- **In-place**: For compatible extension updates
- **Migration**: For breaking extension changes (may require blue-green)

#### D. Unified Upgrade Examples

**Example 1: Patch Release (Low Risk)**
```bash
# v1.2.3 → v1.2.4 (bug fixes, security patches)
helm upgrade documentdb-operator ./documentdb-chart --version v1.2.4

# Components updated:
# - operator: v1.2.3 → v1.2.4
# - gateway: v1.2.3 → v1.2.4  
# - postgres: 16.2-v1.2.3 → 16.2-v1.2.4
# - sidecar-injector: v1.2.3 → v1.2.4
# - cnpg: no change (still v0.24.0)
```

**Example 2: Minor Release (Medium Risk)**  
```bash
# v1.2.4 → v1.3.0 (new features, minor PostgreSQL bump)
helm upgrade documentdb-operator ./documentdb-chart --version v1.3.0

# Components updated:
# - operator: v1.2.4 → v1.3.0
# - gateway: v1.2.4 → v1.3.0
# - postgres: 16.2-v1.2.4 → 16.3-v1.3.0 (minor PG upgrade)
# - sidecar-injector: v1.2.4 → v1.3.0
# - cnpg: v0.24.0 → v0.26.0 (updated for new features)
```

**Example 3: Major Release (High Risk)**
```bash
# v1.3.0 → v2.0.0 (major PostgreSQL upgrade, breaking changes)
# Requires blue-green deployment for PostgreSQL migration

# Step 1: Deploy green cluster with v2.0.0
kubectl apply -f documentdb-v2-green-cluster.yaml

# Step 2: Data migration (PostgreSQL 16.x → 17.x)
# Step 3: Switch traffic after validation
# Step 4: Monitor and cleanup blue cluster

# Components updated:
# - operator: v1.3.0 → v2.0.0
# - gateway: v1.3.0 → v2.0.0
# - postgres: 16.3-v1.3.0 → 17.1-v2.0.0 (major PG upgrade)
# - sidecar-injector: v1.3.0 → v2.0.0
# - cnpg: v0.26.0 → v0.28.0
```

#### E. Unified Versioning Benefits and Rationale

**Why Unified Versioning?**

1. **Operational Simplicity**
   - Single version to track across all environments
   - No need to manage complex component version matrices
   - Simplified documentation and support

2. **Guaranteed Compatibility**  
   - All components tested together as a complete system
   - Eliminates integration issues between component versions
   - Reduces testing matrix complexity

3. **Atomic Operations**
   - All components upgrade together or not at all
   - Consistent state across all environments
   - Simplified rollback procedures

4. **Clear Release Management**
   - Single release pipeline for all components
   - Clear release notes covering all component changes
   - Unified changelog and documentation

**Version Tagging Strategy:**
```bash
# All component images tagged with same DocumentDB version
ghcr.io/microsoft/documentdb-operator:v1.3.0
ghcr.io/microsoft/documentdb-gateway:v1.3.0  
ghcr.io/microsoft/documentdb-sidecar-injector:v1.3.0
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

#### F. Unified Rollback Strategy

**With unified versioning, rollback must happen in the reverse order of the upgrade sequence to maintain system stability.**

##### Rollback Order (Reverse of Upgrade Sequence)

1. **CNPG Clusters** - Restart pods to get previous images
2. **Sidecar Injector** - Rollback injection webhook 
3. **DocumentDB Operator** - Rollback CRDs, controller, webhooks, RBAC
4. **CNPG Operator** - Rollback to previous version (if it was upgraded)

##### Hash-Based Change Detection

**Avoid Unnecessary Rollbacks/Updates with Component SHA Matching:**

Each component tracks its current configuration hash to determine if rollback/update is needed:

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

##### Automated Unified Rollback with Change Detection

**Selective Component Rollback (Only Changed Components):**

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
            rollback.documentdb.microsoft.com/version="$PREVIOUS_REVISION" \
            rollback.documentdb.microsoft.com/reason="component-change-detected" \
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
            mongosh "mongodb://$service_name.$namespace.svc.cluster.local:27017/test" --eval "
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

echo "=== Rollback Complete ==="
echo "All components have been rolled back to revision $PREVIOUS_REVISION"
```

##### Manual Component Rollback (Emergency)

**If automatic rollback fails, manual rollback in proper order:**

```bash
# Emergency Manual Rollback Procedure

# Step 1: Manual Helm rollback
helm rollback documentdb-operator $PREVIOUS_REVISION -n documentdb-system

# Step 2: If Helm rollback fails, manual operator rollback
kubectl patch deployment documentdb-operator -n documentdb-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"documentdb-operator","image":"ghcr.io/microsoft/documentdb-operator:v1.2.3"}]}}}}'

# Step 3: Manual CNPG operator rollback (if needed)
kubectl patch deployment cnpg-controller-manager -n cnpg-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"manager","image":"ghcr.io/cloudnative-pg/cloudnative-pg:1.24.0"}]}}}}'

# Step 4: Manual sidecar injector rollback
kubectl patch deployment sidecar-injector -n cnpg-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"sidecar-injector","image":"ghcr.io/microsoft/documentdb-sidecar-injector:v1.2.3"}]}}}}'

# Step 5: Force rolling restart of all CNPG clusters
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  kubectl delete pods -l cnpg.io/cluster=$cluster -n $namespace
done
```

##### Change Detection Best Practices

**Hash-Based Component Tracking:**

1. **Automatic Hash Generation**: Component hashes are automatically generated during each Helm deployment and stored in ConfigMaps for tracking
2. **Cross-Revision Comparison**: Before any rollback/upgrade, hashes are compared to identify which components actually changed
3. **Selective Operations**: Only changed components undergo rollback/restart operations, reducing disruption and time
4. **Change Tracking**: All rollback operations record which components changed and which were skipped for audit purposes

**Benefits of Change Detection:**
- **Reduced Downtime**: Skip unnecessary restarts for unchanged components
- **Faster Rollbacks**: Only roll back components that actually changed
- **Better Observability**: Clear visibility into what changed between versions
- **Safer Operations**: Avoid touching stable, unchanged components

**Change Detection Configuration:**

```bash
# Enable change detection by default in your rollback scripts
export ENABLE_CHANGE_DETECTION=true

# Force rollback of all components (bypass change detection)
export FORCE_FULL_ROLLBACK=false

# Retention policy for component hash ConfigMaps (keep last 10 revisions)
export HASH_RETENTION_COUNT=10
```

**Hash Storage and Cleanup:**

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

##### Rollback Validation Checklist

**Post-Rollback Verification (with Change Detection):**
- [ ] Change detection correctly identified modified components
- [ ] Only changed components were rolled back (verify in rollback summary)
- [ ] Unchanged components maintained their existing versions/state
- [ ] All operator deployments are healthy and running target versions
- [ ] CNPG clusters are in Ready state with target DocumentDB/Gateway images  
- [ ] MongoDB connectivity is working across all clusters
- [ ] No errors in operator or cluster logs
- [ ] Component versions are consistent with target revision
- [ ] Helm history shows successful rollback to target revision
- [ ] Component hash ConfigMaps updated with rollback information

**Rollback Recovery Time Objectives:**
- **Operator Rollback**: 2-5 minutes (Helm rollback)
- **Cluster Image Rollback**: 5-15 minutes (rolling restart of all clusters)
- **Total Rollback Time**: 10-20 minutes (depending on cluster count)

##### Rollback Prevention

**Best Practices to Minimize Rollback Need:**
1. **Staging Validation**: Always test unified upgrades in staging first
2. **Progressive Rollout**: Upgrade non-production environments first
3. **Backup Strategy**: Ensure recent backups before major upgrades
4. **Monitoring**: Set up alerts for upgrade issues to enable quick rollback
5. **Blue-Green**: Use blue-green deployments for high-risk upgrades

## Legacy Component Upgrade Strategies (Deprecated)

**Note**: The following individual component upgrade strategies are **deprecated** with the introduction of unified versioning. They are preserved for reference only.

<details>
<summary>Click to expand legacy individual component upgrade strategies</summary>

### 2. Sidecar Injector Upgrade Strategy (DEPRECATED)

**CRD Versioning Strategy:**
```yaml
# documentdb_types.go - Version migration
// +kubebuilder:storageversion
type DocumentDBSpec struct {
    // v1 fields
    NodeCount int `json:"nodeCount"`
    
    // v2 fields with backward compatibility
    ExposeViaService *ExposeViaService `json:"exposeViaService,omitempty"`
    
    // Deprecated fields (maintain for backward compatibility)
    // +optional
    PublicLoadBalancer *PublicLoadBalancer `json:"publicLoadBalancer,omitempty"`
}

**CRD Version Conversion Webhook**

**Hub Version Concept:**
- `conversion.Hub` is an interface that marks the "hub" version (typically the latest/storage version)
- All CRD versions convert to/from the hub version instead of direct version-to-version conversions
- This creates a hub-and-spoke model: `v1 ↔ v2(hub) ↔ v3`

**Pseudo Code Implementation:**

```go
// Hub version (v2) - implements conversion.Hub interface
type DocumentDBV2 struct {
    // Latest version fields
    Spec DocumentDBSpecV2 `json:"spec"`
}
func (*DocumentDBV2) Hub() {} // Marks this as hub version

// Older version (v1) - implements conversion methods
type DocumentDBV1 struct {
    Spec DocumentDBSpecV1 `json:"spec"`
}

// ConvertTo: v1 → v2 (hub)
func (src *DocumentDBV1) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*DocumentDBV2)
    
    // 1. Copy unchanged fields
    dst.ObjectMeta = src.ObjectMeta
    dst.Status = src.Status
    
    // 2. Migrate field changes
    if src.Spec.PublicLoadBalancer.Enabled {
        dst.Spec.ExposeViaService = &ExposeViaService{
            ServiceType: "LoadBalancer"
        }
    }
    
    // 3. Set defaults for new fields
    dst.Spec.NewV2Field = "default_value"
    
    return nil
}

// ConvertFrom: v2 (hub) → v1
func (dst *DocumentDBV1) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*DocumentDBV2)
    
    // 1. Copy unchanged fields
    dst.ObjectMeta = src.ObjectMeta
    dst.Status = src.Status
    
    // 2. Reverse field migrations
    if src.Spec.ExposeViaService.ServiceType == "LoadBalancer" {
        dst.Spec.PublicLoadBalancer = &PublicLoadBalancer{
            Enabled: true
        }
    }
    
    // 3. Note: New v2 fields are lost during downgrade
    
    return nil
}
```

**Why Separate CRD Upgrade?**
- **Helm Limitations**: Helm doesn't handle CRD upgrades well, especially with version migrations
- **Safety**: Allows validation of CRD changes before operator upgrade
- **Rollback**: Easier to rollback CRDs independently if issues occur
- **Version Migration**: Required for proper conversion webhook setup

**Alternative: Include CRDs in Helm (Not Recommended)**
```bash
# This approach has limitations and is not recommended for complex CRD changes
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --install-crds  # Limited support for CRD upgrades
```

**Limitations of Helm CRD handling:**
- No automatic CRD upgrades on `helm upgrade`
- Limited support for version migrations
- Difficult rollback scenarios
- No validation of CRD compatibility


#### E. Upgrade Validation and Testing

**Pre-Upgrade Validation:**
```bash
#!/bin/bash
# comprehensive-pre-upgrade-validation.sh

echo "=== DocumentDB Operator Upgrade Pre-Validation ==="

# 1. Check CNPG operator health
echo "Checking CNPG operator health..."
kubectl get deployment cnpg-controller-manager -n cnpg-system || exit 1

# 2. Validate existing DocumentDB resources
echo "Validating existing DocumentDB resources..."
kubectl get documentdb -A -o yaml > /tmp/documentdb-backup.yaml
if [ ! -s /tmp/documentdb-backup.yaml ]; then
    echo "Warning: No DocumentDB resources found"
fi

# 3. Check CNPG cluster status
echo "Checking CNPG cluster status..."
kubectl get clusters.postgresql.cnpg.io -A -o wide

# 4. Verify webhook configurations
echo "Verifying webhook configurations..."
kubectl get validatingwebhookconfiguration | grep -E "(cnpg|documentdb)"
kubectl get mutatingwebhookconfiguration | grep -E "(cnpg|documentdb)"

# 5. Check resource quotas and limits
echo "Checking resource availability..."
kubectl top nodes
kubectl get limitrange -A

# 6. Validate CRD versions
echo "Validating CRD versions..."
kubectl get crd documentdbs.db.microsoft.com -o jsonpath='{.spec.versions[*].name}'

# 7. Test operator responsiveness
echo "Testing operator responsiveness..."
kubectl get pods -n documentdb-system -l app.kubernetes.io/name=documentdb-operator

echo "=== Pre-validation complete ==="
```

**Post-Upgrade Validation:**
```bash
#!/bin/bash
# comprehensive-post-upgrade-validation.sh

echo "=== DocumentDB Operator Upgrade Post-Validation ==="

# 1. Verify operator deployment
echo "Checking operator deployment..."
kubectl rollout status deployment/documentdb-operator -n documentdb-system

# 2. Check CNPG integration
echo "Verifying CNPG integration..."
kubectl get clusters.postgresql.cnpg.io -A -o wide

# 3. Test DocumentDB resource reconciliation
echo "Testing DocumentDB resource reconciliation..."
kubectl get documentdb -A -o wide

# 4. Verify sidecar injection
echo "Checking sidecar injection..."
kubectl get pods -l app.kubernetes.io/name=documentdb-cluster -o jsonpath='{.items[*].spec.containers[*].name}'

# 5. Test MongoDB connectivity
echo "Testing MongoDB connectivity..."
# Add your specific MongoDB connection test here

# 6. Check operator logs for errors
echo "Checking operator logs..."
kubectl logs -n documentdb-system -l app.kubernetes.io/name=documentdb-operator --tail=50

# 7. Validate webhooks are functioning
echo "Validating webhooks..."
kubectl get validatingwebhookconfiguration documentdb-sidecar-injector -o yaml

echo "=== Post-validation complete ==="
```

#### F. Rollback Procedures

**Automated Rollback:**
```bash
#!/bin/bash
# automated-rollback.sh

echo "=== Initiating DocumentDB Operator Rollback ==="

# 1. Get current revision
CURRENT_REVISION=$(helm history documentdb-operator -n documentdb-system --max 1 -o json | jq -r '.[0].revision')
PREVIOUS_REVISION=$((CURRENT_REVISION - 1))

echo "Rolling back from revision $CURRENT_REVISION to $PREVIOUS_REVISION"

# 2. Perform Helm rollback
helm rollback documentdb-operator $PREVIOUS_REVISION -n documentdb-system --wait

# 3. Verify rollback
kubectl rollout status deployment/documentdb-operator -n documentdb-system

# 4. Check CNPG clusters are still healthy
kubectl get clusters.postgresql.cnpg.io -A -o wide

# 5. Verify DocumentDB resources
kubectl get documentdb -A -o wide

echo "=== Rollback complete ==="
```


### 2. Sidecar Injector Upgrade Strategy

The DocumentDB Sidecar Injector is a **stateless** webhook that automatically injects the DocumentDB Gateway container into CNPG PostgreSQL pods. It runs as a CNPG plugin and focuses purely on injection logic and lifecycle management.

#### A. Sidecar Injector Architecture

**Component Overview:**
- **Injector Service**: CNPG plugin service running on port 9090
- **Deployment**: Single replica deployment in `cnpg-system` namespace  
- **TLS Certificates**: Mutual TLS between injector and CNPG operator
- **Injection Logic**: Code that determines when and how to inject gateway containers
- **Lifecycle Management**: Handles container lifecycle events and coordination

**Key Dependencies:**
```yaml
# From values.yaml - Sidecar Injector Image Only
image:
  sidecarinjector:
    repository: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector
    tag: "001"  # Sidecar injector version
```

**Note**: Gateway image configuration is handled separately in Section 3 (Gateway Upgrade Strategy).

#### B. Upgrade Scenarios

##### Sidecar Injector Code Update
**Trigger**: New injection logic, bug fixes, webhook configuration changes, or TLS handling improvements
**Impact**: Affects new pod creation immediately; existing pods require manual recreation to benefit from new injector logic
**Risk**: Medium - Injection failures affect new PostgreSQL pods; pod recreation uses CNPG rolling restarts (no service disruption with multiple replicas)

**Common Update Types:**
- Injection logic improvements
- Webhook security enhancements  
- TLS certificate management updates
- CNPG plugin API compatibility updates
- Lifecycle management refinements

**Important**: Since the sidecar injector only affects **new pod creation**, existing pods will continue running with the old injection configuration until they are recreated. For critical injector updates (security fixes, compatibility updates), you must recreate existing pods to apply the new injection logic.

#### C. Upgrade Strategy

**Rolling Update (Recommended)**

Since the sidecar injector is a **stateless** component, rolling updates provide the optimal balance of safety and simplicity:

```bash
# Step 1: Update sidecar injector image in values.yaml
cat <<EOF > values-update.yaml
image:
  sidecarinjector:
    repository: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector
    tag: "002"  # New injector version
EOF

# Step 2: Upgrade via Helm (Rolling Update)
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --values values-update.yaml \
  --wait \
  --timeout 300s

# Step 3: Verify injector deployment rollout
kubectl rollout status deployment/sidecar-injector -n cnpg-system

# Step 4: Verify injector webhook is active
kubectl get mutatingwebhookconfiguration documentdb-sidecar-injector

# Step 5: Test injection on new pods (verify new injector logic works)
kubectl apply -f - <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: test-new-injection
spec:
  instances: 1
  postgresql:
    parameters:
      shared_preload_libraries: "documentdb"
EOF

# Wait for pod creation and verify new injection logic
kubectl wait --for=condition=Ready pod -l cnpg.io/cluster=test-new-injection --timeout=300s
kubectl describe pod -l cnpg.io/cluster=test-new-injection | grep -A5 "gateway"

# Step 6: Recreate existing pods to apply new injector logic
# This step is REQUIRED for existing pods to benefit from the new injector
echo "Recreating existing DocumentDB pods to apply new injector logic..."

# Option A: Rolling restart of existing CNPG clusters (Recommended)
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  echo "Restarting cluster: $cluster in namespace: $namespace"
  
  # Trigger rolling restart via CNPG
  kubectl annotate clusters.postgresql.cnpg.io $cluster -n $namespace \
    cnpg.io/reloadedAt="$(date -Iseconds)"
  
  # Wait for restart to complete
  kubectl wait --for=condition=Ready clusters.postgresql.cnpg.io/$cluster -n $namespace --timeout=600s
done

# Option B: Manual pod deletion (Alternative approach)
# kubectl delete pods -l cnpg.io/cluster --all-namespaces --wait=false
# kubectl wait --for=condition=Ready pod -l cnpg.io/cluster --all-namespaces --timeout=600s

# Step 7: Verify all pods now have the new injection configuration
kubectl get pods -l cnpg.io/cluster --all-namespaces -o custom-columns=\
"NAMESPACE:.metadata.namespace,NAME:.metadata.name,CONTAINERS:.spec.containers[*].name"

# Clean up test cluster
kubectl delete cluster test-new-injection
```

**Important Considerations for Pod Recreation:**
- **Service Continuity**: CNPG performs rolling restarts to maintain service availability (no downtime with multiple replicas)
- **Data Persistence**: Pod recreation does not affect PostgreSQL data (stored in PVCs)
- **Connection Handling**: With multiple replicas, client connections can be handled by remaining pods during rolling restart
- **Single Replica Clusters**: Brief connection interruption possible during pod restart (consider scaling up temporarily)
- **Monitoring**: Monitor application health during pod recreation process

#### D. Validation

**Post-Upgrade Validation:**
```bash
# Check injector pod logs for errors
kubectl logs -l app=sidecar-injector -n cnpg-system --tail=50

# Verify webhook configuration is updated
kubectl get mutatingwebhookconfiguration documentdb-sidecar-injector -o yaml

# Validate injection logic on newly created pods
kubectl apply -f - <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: validate-injection
spec:
  instances: 1
  postgresql:
    parameters:
      shared_preload_libraries: "documentdb"
EOF

# Wait for pod creation and verify new injection worked
kubectl wait --for=condition=Ready pod -l cnpg.io/cluster=validate-injection --timeout=300s
kubectl get pod -l cnpg.io/cluster=validate-injection -o jsonpath='{.items[0].spec.containers[*].name}'

# Verify recreated existing pods have new injection configuration
kubectl get pods -l cnpg.io/cluster --all-namespaces -o custom-columns=\
"NAMESPACE:.metadata.namespace,NAME:.metadata.name,CREATED:.metadata.creationTimestamp"

# Check that recreated pods have expected container configuration
for pod in $(kubectl get pods -l cnpg.io/cluster --all-namespaces -o jsonpath='{.items[*].metadata.name}'); do
  echo "Pod: $pod"
  kubectl get pod $pod -o jsonpath='{.spec.containers[*].name}' && echo
done

# Clean up validation resources
kubectl delete cluster validate-injection
```


**Rollback Strategy:**
```bash
# Helm rollback to previous injector version
helm rollback documentdb-operator --namespace documentdb-system

# Or manual image rollback
kubectl patch deployment sidecar-injector -n cnpg-system -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"sidecar-injector","image":"ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector:001"}]}}}}'
```

### 3. Gateway Image Upgrade Strategy

The Gateway container is **stateless** and acts as a protocol translator between MongoDB clients and PostgreSQL. Gateway image upgrades are handled through the **sidecar injector**, which injects the specified gateway image version into CNPG PostgreSQL pods.

#### A. Gateway Upgrade Architecture

**Gateway Image Injection Flow:**
1. **Configuration Update**: Gateway image version specified in DocumentDB operator configuration
2. **Sidecar Injector**: Reads gateway image configuration and injects into new pods
3. **Pod Recreation**: Existing pods must be recreated to get the new gateway image
4. **Rolling Restart**: CNPG performs rolling restart to maintain service availability

**Key Components:**
- **Gateway Image Configuration**: Stored in DocumentDB operator values/configmap
- **Sidecar Injector**: CNPG plugin that handles gateway container injection
- **CNPG Rolling Restart**: Maintains service continuity during updates

#### B. Gateway Image Upgrade Process

**Step-by-Step Gateway Upgrade:**

```bash
# Step 1: Update gateway image version in values.yaml
cat <<EOF > gateway-update-values.yaml
image:
  gateway:
    repository: ghcr.io/microsoft/documentdb/documentdb-local
    tag: "17"  # New gateway version
  # Sidecar injector version can remain the same if only gateway image changes
  sidecarinjector:
    repository: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector
    tag: "001"
EOF

# Step 2: Upgrade DocumentDB operator with new gateway image configuration
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --values gateway-update-values.yaml \
  --wait \
  --timeout 600s

# Step 3: Verify sidecar injector has new gateway configuration
kubectl get configmap documentdb-gateway-config -n documentdb-system -o yaml | grep "image:"

# Step 4: Test gateway injection on new pods
kubectl apply -f - <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: test-gateway-upgrade
spec:
  instances: 1
  postgresql:
    parameters:
      shared_preload_libraries: "documentdb"
EOF

# Wait for pod creation and verify new gateway image
kubectl wait --for=condition=Ready pod -l cnpg.io/cluster=test-gateway-upgrade --timeout=300s
kubectl get pod -l cnpg.io/cluster=test-gateway-upgrade -o jsonpath='{.items[0].spec.containers[?(@.name=="documentdb-gateway")].image}'

# Step 5: Upgrade existing clusters with new gateway image
# This requires pod recreation since gateway is injected at pod creation time
echo "Upgrading existing DocumentDB clusters with new gateway image..."

for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  echo "Upgrading gateway in cluster: $cluster (namespace: $namespace)"
  
  # Trigger rolling restart to get new gateway image
  kubectl annotate clusters.postgresql.cnpg.io $cluster -n $namespace \
    cnpg.io/reloadedAt="$(date -Iseconds)" \
    upgrade.documentdb.microsoft.com/gateway-version="17"
  
  # Wait for rolling restart to complete
  kubectl wait --for=condition=Ready clusters.postgresql.cnpg.io/$cluster -n $namespace --timeout=600s
  
  # Verify new gateway image is running
  kubectl get pods -l cnpg.io/cluster=$cluster -n $namespace -o jsonpath='{.items[*].spec.containers[?(@.name=="documentdb-gateway")].image}'
done

# Step 6: Validate gateway functionality with new image
echo "Validating gateway functionality..."
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  service_name="${cluster}-rw"
  
  # Test MongoDB connectivity through new gateway
  kubectl run mongodb-test-$cluster --rm -i --tty --image=mongo:7 -- \
    mongosh "mongodb://$service_name.$namespace.svc.cluster.local:27017/test" --eval "
      db.gateway_upgrade_test.insertOne({
        test: 'gateway_upgrade', 
        version: '17', 
        timestamp: new Date()
      });
      print('Gateway upgrade test completed for cluster: $cluster');
    "
done

# Clean up test cluster
kubectl delete cluster test-gateway-upgrade
```

#### C. Gateway Configuration Update

**Update Gateway Image in Helm Values:**
```yaml
# values.yaml - Gateway image version update
image:
  gateway:
    repository: ghcr.io/microsoft/documentdb/documentdb-local
    tag: "17"  # New gateway version
```

#### D. Gateway Upgrade Validation

**Pre-Upgrade Validation:**
```bash
#!/bin/bash
# gateway-upgrade-pre-validation.sh

echo "=== Gateway Image Upgrade Pre-Validation ==="

# 1. Check current gateway image versions across clusters
echo "Current gateway image versions:"
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  current_image=$(kubectl get pods -l cnpg.io/cluster=$cluster -n $namespace -o jsonpath='{.items[0].spec.containers[?(@.name=="documentdb-gateway")].image}' 2>/dev/null || echo "No gateway found")
  echo "  Cluster: $cluster (namespace: $namespace) - Gateway: $current_image"
done

# 2. Verify sidecar injector is healthy
echo "Checking sidecar injector status..."
kubectl get deployment sidecar-injector -n cnpg-system -o wide

# 3. Check gateway image availability
echo "Verifying new gateway image availability..."
NEW_GATEWAY_IMAGE="ghcr.io/microsoft/documentdb/documentdb-local:17"
docker manifest inspect $NEW_GATEWAY_IMAGE > /dev/null 2>&1 && echo "✅ Gateway image $NEW_GATEWAY_IMAGE is available" || echo "❌ Gateway image $NEW_GATEWAY_IMAGE not found"

# 4. Test MongoDB connectivity on existing clusters
echo "Testing MongoDB connectivity before upgrade..."
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  service_name="${cluster}-rw"
  
  kubectl run pre-upgrade-test-$cluster --rm -i --tty --timeout=30s --image=mongo:7 -- \
    mongosh "mongodb://$service_name.$namespace.svc.cluster.local:27017/test" --eval "
      db.pre_upgrade_test.insertOne({test: 'pre_upgrade', timestamp: new Date()});
      print('Pre-upgrade connectivity test passed for cluster: $cluster');
    " 2>/dev/null || echo "❌ Connectivity test failed for cluster: $cluster"
done

echo "=== Pre-validation complete ==="
```

**Post-Upgrade Validation:**
```bash
#!/bin/bash
# gateway-upgrade-post-validation.sh

echo "=== Gateway Image Upgrade Post-Validation ==="

# 1. Verify all clusters have new gateway image
echo "Verifying gateway image versions after upgrade:"
EXPECTED_GATEWAY="ghcr.io/microsoft/documentdb/documentdb-local:17"
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  
  # Check all pods in the cluster
  kubectl get pods -l cnpg.io/cluster=$cluster -n $namespace -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[?(@.name=="documentdb-gateway")].image}{"\n"}{end}' | while read pod_name image; do
    if [ "$image" = "$EXPECTED_GATEWAY" ]; then
      echo "✅ Pod $pod_name has correct gateway image: $image"
    else
      echo "❌ Pod $pod_name has incorrect gateway image: $image (expected: $EXPECTED_GATEWAY)"
    fi
  done
done

# 2. Test gateway functionality and performance
echo "Testing gateway functionality with new image..."
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  service_name="${cluster}-rw"
  
  kubectl run post-upgrade-test-$cluster --rm -i --tty --timeout=60s --image=mongo:7 -- \
    mongosh "mongodb://$service_name.$namespace.svc.cluster.local:27017/test" --eval "
      // Test basic operations
      db.post_upgrade_test.insertOne({
        test: 'post_upgrade',
        gateway_version: '17',
        timestamp: new Date()
      });
      
      // Test query performance
      var start = new Date();
      db.post_upgrade_test.findOne({test: 'post_upgrade'});
      var end = new Date();
      print('Query response time: ' + (end - start) + 'ms');
      
      // Test aggregation
      db.post_upgrade_test.aggregate([
        {\$match: {test: 'post_upgrade'}},
        {\$count: 'total'}
      ]);
      
      print('Post-upgrade functionality test passed for cluster: $cluster');
    " || echo "❌ Functionality test failed for cluster: $cluster"
done

# 3. Check gateway container logs for errors
echo "Checking gateway container logs for errors..."
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  echo "Gateway logs for cluster $cluster:"
  kubectl logs -l cnpg.io/cluster=$cluster -n $namespace -c documentdb-gateway --tail=10 | grep -E "(ERROR|WARN|FATAL)" || echo "  No errors found"
done

echo "=== Post-validation complete ==="
```

#### E. Gateway Rollback Strategy

**Automated Gateway Rollback:**
```bash
#!/bin/bash
# gateway-rollback.sh

echo "=== Initiating Gateway Image Rollback ==="

# 1. Identify previous gateway version
PREVIOUS_GATEWAY_IMAGE="ghcr.io/microsoft/documentdb/documentdb-local:16"

# 2. Update operator configuration with previous gateway image
cat <<EOF > gateway-rollback-values.yaml
image:
  gateway:
    repository: ghcr.io/microsoft/documentdb/documentdb-local
    tag: "16"  # Previous gateway version
EOF

# 3. Perform Helm rollback or update with previous image
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --values gateway-rollback-values.yaml \
  --wait \
  --timeout 600s

# 4. Recreate pods with previous gateway image
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  echo "Rolling back gateway in cluster: $cluster"
  
  kubectl annotate clusters.postgresql.cnpg.io $cluster -n $namespace \
    cnpg.io/reloadedAt="$(date -Iseconds)" \
    upgrade.documentdb.microsoft.com/gateway-rollback="16"
  
  kubectl wait --for=condition=Ready clusters.postgresql.cnpg.io/$cluster -n $namespace --timeout=600s
done

# 5. Verify rollback success
echo "Verifying gateway rollback..."
for cluster in $(kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{.items[*].metadata.name}'); do
  namespace=$(kubectl get clusters.postgresql.cnpg.io $cluster -A -o jsonpath='{.items[0].metadata.namespace}')
  current_image=$(kubectl get pods -l cnpg.io/cluster=$cluster -n $namespace -o jsonpath='{.items[0].spec.containers[?(@.name=="documentdb-gateway")].image}')
  
  if [ "$current_image" = "$PREVIOUS_GATEWAY_IMAGE" ]; then
    echo "✅ Cluster $cluster successfully rolled back to gateway $current_image"
  else
    echo "❌ Cluster $cluster rollback failed. Current image: $current_image"
  fi
done

echo "=== Gateway rollback complete ==="
```

### 4. CNPG Operator Upgrade Strategy

The CloudNativePG (CNPG) operator manages PostgreSQL clusters and is a critical dependency for DocumentDB. CNPG upgrades are **tightly coupled** with DocumentDB operator versions to ensure compatibility and stability.

**Important**: CNPG operator upgrades are **not** available as standalone upgrades for customers. The CNPG version is bundled with and upgraded automatically as part of DocumentDB operator upgrades.

#### A. Version Coupling Policy

**CNPG-DocumentDB Version Binding:**
- Each DocumentDB operator version is tested and certified with a specific CNPG version
- CNPG upgrades are only available through DocumentDB operator upgrades
- This ensures full compatibility testing and reduces upgrade complexity for customers

**Supported Upgrade Path:**
```
DocumentDB v1.2.0 + CNPG v0.24.0 
         ↓
DocumentDB v1.3.0 + CNPG v0.26.0 
         ↓
DocumentDB v1.4.0 + CNPG v0.28.0
```

#### B. CNPG Upgrade via DocumentDB Operator

##### Helm Dependency Management (Only Supported Method)
```yaml
# documentdb-chart/Chart.yaml
dependencies:
  - name: cloudnative-pg
    version: "0.26.0"  # Locked to DocumentDB operator version
    repository: https://cloudnative-pg.github.io/charts
    condition: cnpg.enabled
```

**Upgrade Process:**
```bash
# CNPG is upgraded automatically as part of DocumentDB operator upgrade
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --wait \
  --timeout 900s
```

**Note**: Customers cannot and should not upgrade CNPG independently. Any attempt to do so may result in:
- Incompatibility issues between DocumentDB and CNPG
- Unsupported configuration states
- Potential data corruption or service disruption

#### C. CNPG Dependency Management

**Helm Dependency Update Process:**
```bash
# Step 1: Update CNPG dependency (performed automatically during DocumentDB upgrade)
helm dependency update ./documentdb-chart

# Step 2: Verify CNPG chart version in dependencies
helm dependency list ./documentdb-chart

# Expected output:
# NAME            VERSION  REPOSITORY                              STATUS
# cloudnative-pg  0.26.0   https://cloudnative-pg.github.io/charts ok
```

**CNPG Upgrade Validation Process:**
```bash
# Step 1: Validate CNPG CRDs before upgrade
kubectl get crd clusters.postgresql.cnpg.io -o jsonpath='{.spec.versions[*].name}'

# Step 2: Check existing CNPG cluster health
kubectl get clusters.postgresql.cnpg.io -A -o wide

# Step 3: Verify CNPG controller status
kubectl get deployment cnpg-controller-manager -n cnpg-system

# Step 4: Validate CNPG webhooks
kubectl get validatingwebhookconfiguration cnpg-validating-webhook-configuration
kubectl get mutatingwebhookconfiguration cnpg-mutating-webhook-configuration

# Step 5: Check CNPG operator logs for errors
kubectl logs -n cnpg-system -l app.kubernetes.io/name=cloudnative-pg --tail=50
```

**Troubleshooting CNPG Dependency Issues:**
```bash
# If CNPG dependency update fails
rm -rf ./documentdb-chart/charts/cloudnative-pg-*.tgz
rm -f ./documentdb-chart/Chart.lock
helm dependency update ./documentdb-chart

# If CNPG version conflicts occur
helm dependency build ./documentdb-chart --skip-refresh

# Validate CNPG compatibility matrix
kubectl get deployment cnpg-controller-manager -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}'
```

#### D. CNPG Version Compatibility Validation

**Automated Compatibility Check:**
```bash
#!/bin/bash
# validate-cnpg-compatibility.sh

CNPG_VERSION=$1
DOCUMENTDB_VERSION=$(helm list -n documentdb-system -o json | jq -r '.[] | select(.name=="documentdb-operator") | .app_version')

echo "=== CNPG-DocumentDB Compatibility Validation ==="
echo "Validating CNPG $CNPG_VERSION compatibility with DocumentDB $DOCUMENTDB_VERSION"

# Check if DocumentDB operator is installed
if [ -z "$DOCUMENTDB_VERSION" ]; then
    echo "❌ DocumentDB operator not found. Install DocumentDB operator first."
    exit 1
fi

# Check CRD compatibility
echo "Checking CRD versions..."
CNPG_CRD_VERSIONS=$(kubectl get crd clusters.postgresql.cnpg.io -o jsonpath='{.spec.versions[*].name}' 2>/dev/null || echo "")
if [ -z "$CNPG_CRD_VERSIONS" ]; then
    echo "❌ CNPG CRDs not found"
    exit 1
fi
echo "Available CNPG CRD versions: $CNPG_CRD_VERSIONS"

# Validate API version compatibility matrix
case $CNPG_VERSION in
    "0.24."*)
        if [[ $DOCUMENTDB_VERSION == "1.2."* ]]; then
            echo "✅ Compatible: DocumentDB $DOCUMENTDB_VERSION + CNPG $CNPG_VERSION"
        else
            echo "❌ Incompatible: DocumentDB $DOCUMENTDB_VERSION requires CNPG 0.24.x"
            exit 1
        fi
        ;;
    "0.26."*)
        if [[ $DOCUMENTDB_VERSION == "1.3."* ]]; then
            echo "✅ Compatible: DocumentDB $DOCUMENTDB_VERSION + CNPG $CNPG_VERSION"
        else
            echo "❌ Incompatible: DocumentDB $DOCUMENTDB_VERSION not compatible with CNPG 0.26.x"
            exit 1
        fi
        ;;
    "0.28."*)
        if [[ $DOCUMENTDB_VERSION == "1.4."* ]]; then
            echo "✅ Compatible: DocumentDB $DOCUMENTDB_VERSION + CNPG $CNPG_VERSION"
        else
            echo "❌ Incompatible: DocumentDB $DOCUMENTDB_VERSION not compatible with CNPG 0.28.x"
            exit 1
        fi
        ;;
    *)
        echo "❌ Unknown CNPG version $CNPG_VERSION - check compatibility matrix"
        exit 1
        ;;
esac

# Verify CNPG controller health
echo "Checking CNPG controller health..."
kubectl get deployment cnpg-controller-manager -n cnpg-system -o wide
if [ $? -ne 0 ]; then
    echo "❌ CNPG controller not healthy"
    exit 1
fi

# Check existing CNPG clusters
echo "Checking existing CNPG clusters..."
kubectl get clusters.postgresql.cnpg.io -A -o wide

echo "=== Compatibility validation complete ==="
```

#### E. Integrated DocumentDB + CNPG Upgrade Process

When upgrading the DocumentDB operator, CNPG is automatically upgraded as part of the same Helm operation. This ensures version compatibility and reduces operational complexity.

**Complete Upgrade Flow:**
```bash
# Step 1: Pre-upgrade validation
./validate-cnpg-compatibility.sh $(helm show chart ./documentdb-chart/charts/cloudnative-pg-*.tgz | grep "^version:" | cut -d' ' -f2)

# Step 2: Update Helm dependencies (includes CNPG chart)
helm dependency update ./documentdb-chart

# Step 3: Perform integrated upgrade (DocumentDB + CNPG)
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --wait \
  --timeout 900s \
  --atomic

# Step 4: Verify both operators are healthy
kubectl get deployment documentdb-operator -n documentdb-system
kubectl get deployment cnpg-controller-manager -n cnpg-system

# Step 5: Validate DocumentDB clusters are still functional
kubectl get documentdb -A -o wide
kubectl get clusters.postgresql.cnpg.io -A -o wide
```

**Upgrade Sequence (Automatic):**
1. **CNPG CRDs**: Updated first to support new API versions
2. **CNPG Controller**: Upgraded to new version with backward compatibility
3. **DocumentDB CRDs**: Updated with any schema changes
4. **DocumentDB Controller**: Upgraded to work with new CNPG version
5. **Webhooks**: Updated to maintain admission control functionality

**Post-Upgrade Validation:**
```bash
# Verify version alignment
DOCUMENTDB_VERSION=$(kubectl get deployment documentdb-operator -n documentdb-system -o jsonpath='{.spec.template.spec.containers[0].image}')
CNPG_VERSION=$(kubectl get deployment cnpg-controller-manager -n cnpg-system -o jsonpath='{.spec.template.spec.containers[0].image}')

echo "DocumentDB Version: $DOCUMENTDB_VERSION"
echo "CNPG Version: $CNPG_VERSION"

# Test DocumentDB functionality
kubectl get documentdb -A -o wide
mongosh "mongodb://username:password@my-documentdb-service:27017/test" --eval "db.test.findOne()"
```

### 5. PostgreSQL Database Upgrade Strategy

PostgreSQL is **stateful** and contains all persistent application data. This requires careful planning, extensive testing, and robust backup strategies.

#### A. Stateful Upgrade Challenges
- **Data Persistence**: All application data stored in PostgreSQL
- **Migration Complexity**: Schema and data migration between PostgreSQL versions
- **Downtime Risk**: Major upgrades may require significant downtime
- **Rollback Complexity**: Point-in-time recovery required for rollbacks
- **Validation Requirements**: Extensive data integrity validation needed

#### B. PostgreSQL Upgrade Categories

##### Minor Version Upgrades (e.g., 16.2 → 16.3)
**Risk Level**: Low to Medium
**Strategy**: In-place upgrade with rolling restart

```yaml
# For minor PostgreSQL versions
spec:
  postgresql:
    parameters:
      shared_preload_libraries: "documentdb_extension"
    image: "mcr.microsoft.com/documentdb/documentdb:16.3-ext-1.1"
```

**Process:**
1. Create automated backup before upgrade
2. Update image in DocumentDB CR
3. CNPG performs rolling restart
4. Validate data integrity post-upgrade

##### Major Version Upgrades (e.g., 16.x → 17.x)
**Risk Level**: High
**Strategy**: pg_upgrade or dump/restore with extended downtime

```bash
# Step 1: Create comprehensive backup
kubectl exec -it my-documentdb-cluster-1 -- pg_dumpall > full-backup-$(date +%Y%m%d).sql

# Step 2: Test upgrade in staging environment
kubectl apply -f documentdb-test-v15.yaml

# Step 3: Validate DocumentDB extension compatibility with new PostgreSQL
kubectl exec -it test-cluster-1 -- psql -c "SELECT documentdb_extension_version();"

# Step 4: Schedule maintenance window for production upgrade
```

#### C. Data Migration Strategies

##### Strategy 1: CNPG Built-in Upgrade (Recommended for Minor Versions)
```yaml
# CNPG handles PostgreSQL minor upgrades automatically
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
spec:
  imageName: "mcr.microsoft.com/documentdb/documentdb:16.3-ext-1.1"
  # CNPG will handle the upgrade process
```

##### Strategy 2: Blue-Green Migration (Major Versions and High-Risk Upgrades)

**When to Use Blue-Green for PostgreSQL:**
- **Major PostgreSQL versions** (e.g., 16.x → 17.x)
- **Breaking changes** in DocumentDB extension
- **High-risk schema migrations**
- **Production environments** requiring zero-downtime upgrades

**Important**: Blue-Green deployments for PostgreSQL require careful data migration since PostgreSQL contains all persistent application data.

**Data Migration Considerations:**
- **New PVCs Required**: Blue and green clusters cannot share storage due to:
  - Different PostgreSQL versions may have incompatible data formats
  - CNPG cluster names must be unique (blue: `my-documentdb-cluster`, green: `my-documentdb-green-cluster`)
  - PVC naming is tied to StatefulSet names, which are derived from cluster names
- **Data Migration Methods**: pg_dump/restore, pg_upgrade, or logical replication
- **Storage Sizing**: Green cluster should have equal or larger storage capacity
- **Extension Compatibility**: Validate DocumentDB extension works with new PostgreSQL version

**Complete Blue-Green PostgreSQL Upgrade Process:**
```bash
# Step 1: Deploy new DocumentDB cluster with target PostgreSQL version
kubectl apply -f - <<EOF
apiVersion: db.microsoft.com/v1
kind: DocumentDB
metadata:
  name: my-documentdb-green
  namespace: default
spec:
  # Same configuration as blue cluster, but with new PostgreSQL version
  nodeCount: 1
  instancesPerNode: 1
  postgresqlImage: "mcr.microsoft.com/documentdb/documentdb:17.2-ext-1.2"  # New version
  documentDBImage: "mcr.microsoft.com/documentdb/gateway:v2.1.0"
  resource:
    pvcSize: "15Gi"  # Equal or larger storage size
  exposeViaService:
    serviceType: "ClusterIP"  # Internal during migration
  # Green cluster gets new PVCs automatically
EOF

# Step 2: Wait for green cluster to be ready
kubectl wait --for=condition=Ready documentdb/my-documentdb-green --timeout=900s

# Step 3: Verify green cluster PostgreSQL and extension compatibility
kubectl exec -it my-documentdb-green-cluster-1 -- psql -c "
  SELECT version();
  SELECT documentdb_extension_version();
"

# Step 4: Stop writes to blue cluster (maintenance mode)
kubectl patch documentdb my-documentdb -p '{"spec":{"maintenance": true}}'

# Step 5: Perform comprehensive data migration
echo "Starting data migration from blue to green cluster..."

# Create final backup from blue cluster
kubectl exec -it my-documentdb-cluster-1 -- pg_dumpall --clean --verbose > final-migration-backup-$(date +%Y%m%d-%H%M%S).sql

# Restore data to green cluster
kubectl exec -i my-documentdb-green-cluster-1 -- psql < final-migration-backup-$(date +%Y%m%d-%H%M%S).sql

# Step 6: Validate data integrity on green cluster
kubectl exec -it my-documentdb-green-cluster-1 -- psql -c "
  SELECT 
    schemaname,
    tablename,
    n_tup_ins as row_count
  FROM pg_stat_user_tables 
  WHERE schemaname NOT IN ('information_schema', 'pg_catalog');
"

# Step 7: Compare data between blue and green clusters
echo "Validating data migration integrity..."
BLUE_ROW_COUNT=$(kubectl exec -it my-documentdb-cluster-1 -- psql -t -c "SELECT COUNT(*) FROM pg_stat_user_tables;")
GREEN_ROW_COUNT=$(kubectl exec -it my-documentdb-green-cluster-1 -- psql -t -c "SELECT COUNT(*) FROM pg_stat_user_tables;")

echo "Blue cluster tables: $BLUE_ROW_COUNT"
echo "Green cluster tables: $GREEN_ROW_COUNT"

if [ "$BLUE_ROW_COUNT" != "$GREEN_ROW_COUNT" ]; then
    echo "❌ Data migration validation failed - table counts don't match"
    exit 1
fi

# Step 8: Test MongoDB connectivity and functionality on green cluster
mongosh "mongodb://username:password@my-documentdb-green-service:27017/test" --eval "
  // Test basic operations
  db.test.insertOne({migration_test: new Date(), version: 'green'});
  db.test.findOne({migration_test: {\$exists: true}});
  
  // Validate existing data
  print('Document count:', db.test.countDocuments({}));
"

# Step 9: Switch traffic from blue to green
kubectl patch service my-documentdb-service -p '{
  "spec": {
    "selector": {
      "cnpg.io/cluster": "my-documentdb-green-cluster"
    }
  }
}'

# Step 10: Validate green cluster is serving traffic
mongosh "mongodb://username:password@my-documentdb-service:27017/test" --eval "db.test.findOne()"

# Step 11: Monitor green cluster for 24-48 hours before cleanup
echo "✅ Green cluster is now serving traffic with PostgreSQL $(kubectl exec -it my-documentdb-green-cluster-1 -- psql -t -c 'SELECT version();')"
echo "Blue cluster preserved for rollback: my-documentdb-cluster"
echo "Monitor for 24-48 hours before cleanup."
```

**PostgreSQL Blue-Green Rollback Procedure:**
```bash
# Immediate rollback if issues detected during migration
kubectl patch service my-documentdb-service -p '{
  "spec": {
    "selector": {
      "cnpg.io/cluster": "my-documentdb-cluster"
    }
  }
}'

# Re-enable writes on blue cluster
kubectl patch documentdb my-documentdb -p '{"spec":{"maintenance": false}}'

# Verify blue cluster is serving traffic
mongosh "mongodb://username:password@my-documentdb-service:27017/test" --eval "db.test.findOne()"

# Complete rollback (after confirming blue cluster is stable)
kubectl delete documentdb my-documentdb-green
echo "Green cluster removed, blue cluster restored to full operation"
```

**Important**: For detailed data migration procedures, storage verification, and advanced backup/restore strategies, refer to the dedicated backup/restore guide: `docs/designs/backup-restore/backup-restore-guide.md`

### 6. DocumentDB Extension Upgrade Strategy

The DocumentDB extension provides MongoDB-compatible functionality within PostgreSQL. Extension upgrades require careful coordination with PostgreSQL versions and thorough testing of MongoDB API compatibility.

#### A. Extension Upgrade Challenges
- **Schema Modifications**: Extension may alter database schema or add new objects
- **API Compatibility**: MongoDB API changes may affect client applications  
- **Data Migration**: Extension updates may require data structure modifications
- **Version Dependencies**: Extension must be compatible with PostgreSQL version
- **Rollback Complexity**: Extension downgrades are often not supported

#### B. Extension Upgrade Categories

##### Patch Updates (e.g., 1.1.0 → 1.1.1)
**Risk Level**: Low to Medium
**Strategy**: In-place extension update

```sql
-- Check current extension version
SELECT name, default_version, installed_version 
FROM pg_available_extensions 
WHERE name = 'documentdb_extension';

-- Upgrade extension (if supported)
ALTER EXTENSION documentdb_extension UPDATE TO '1.1.1';

-- Verify upgrade success
SELECT documentdb_extension_version();
```

##### Feature Updates (e.g., 1.1.0 → 1.2.0)
**Risk Level**: Medium to High
**Strategy**: Staged rollout with extensive testing

```bash
# Step 1: Test in staging environment
kubectl exec -it staging-cluster-1 -- psql -c "ALTER EXTENSION documentdb_extension UPDATE TO '1.2.0';"

# Step 2: Validate MongoDB API compatibility
mongosh "mongodb://staging-service:27017/test" --eval "
  // Test new features and existing functionality
  db.test.insertOne({test: 'compatibility'});
  db.test.findOne();
"

# Step 3: Schedule maintenance window for production
```

##### Breaking Changes (e.g., 1.x → 2.0)
**Risk Level**: Very High
**Strategy**: Blue-green deployment with data migration

```bash
# Step 1: Deploy new cluster with updated extension
kubectl apply -f - <<EOF
apiVersion: db.microsoft.com/v1
kind: DocumentDB
metadata:
  name: documentdb-v2
spec:
  postgresqlImage: "mcr.microsoft.com/documentdb/documentdb:16.x-ext-2.0"
  # New extension version
EOF

# Step 2: Migrate data with extension-specific procedures
# (Extension-specific migration tools may be required)

# Step 3: Validate MongoDB API compatibility extensively
# Step 4: Switch traffic after thorough validation
```

#### C. Extension-Specific Considerations

**MongoDB API Compatibility:**
- Validate all existing MongoDB operations still work
- Test new MongoDB features introduced by extension
- Verify query performance and behavior consistency

**Schema Migration:**
- Extension may create new system collections
- Existing collections may require schema updates
- Index structures may need modification

**Client Application Impact:**
- MongoDB drivers may need updates
- Application code may require changes for new features
- Connection strings and authentication may be affected

## Upgrade Orchestration

### 1. Upgrade Order (Simplified with Unified Versioning)

**Single Atomic Upgrade**: All components upgrade together through DocumentDB operator version upgrade.

```mermaid
graph TD
    A[DocumentDB Operator v1.3.0] --> B[All Components v1.3.0]
    B --> C[Gateway: v1.3.0]
    B --> D[PostgreSQL: 16.3-v1.3.0] 
    B --> E[Sidecar Injector: v1.3.0]
    B --> F[CNPG: v0.26.0]
```

### 2. Unified Versioning Matrix

**With unified versioning, all components are aligned to a single DocumentDB operator version:**

| DocumentDB Release | Operator | Gateway | PostgreSQL | Extension | Sidecar Injector | CNPG | Release Type |
|-------------------|----------|---------|------------|-----------|------------------|------|--------------|
| **v1.2.3** | v1.2.3 | v1.2.3 | 16.2-v1.2.3 | v1.2.3 | v1.2.3 | v0.24.0 | Current |
| **v1.2.4** | v1.2.4 | v1.2.4 | 16.2-v1.2.4 | v1.2.4 | v1.2.4 | v0.24.0 | Patch |
| **v1.3.0** | v1.3.0 | v1.3.0 | 16.3-v1.3.0 | v1.3.0 | v1.3.0 | v0.26.0 | Minor |
| **v2.0.0** | v2.0.0 | v2.0.0 | 17.1-v2.0.0 | v2.0.0 | v2.0.0 | v0.28.0 | Major |

**Benefits:**
- ✅ **Simplified Operations**: Single version to track
- ✅ **Guaranteed Compatibility**: All components tested together
- ✅ **Atomic Upgrades**: All-or-nothing upgrade approach
- ✅ **Consistent Rollbacks**: Single version rollback

**Important**: Component versions are no longer independently managed. All upgrades go through DocumentDB operator releases.

### 3. Pre-Upgrade Validation

```bash
#!/bin/bash
# Pre-upgrade validation script

# Check DocumentDB resources
kubectl get documentdb -A

# Verify CNPG clusters are healthy
kubectl get clusters.postgresql.cnpg.io -A

# Check operator status
kubectl get deployment documentdb-operator -n documentdb-system

# Validate webhook configuration
kubectl get validatingwebhookconfiguration documentdb-sidecar-injector
```

### 4. Post-Upgrade Validation

```bash
#!/bin/bash
# Post-upgrade validation script

# Test MongoDB connection
mongosh "mongodb://username:password@documentdb-service:27017/test"

# Check pod status
kubectl get pods -l app=documentdb-cluster

# Verify gateway and postgres are running
kubectl logs -l app=documentdb-cluster -c gateway
kubectl logs -l app=documentdb-cluster -c postgres
```

## Rollback Strategy

### 1. Operator Rollback
```bash
# Helm rollback
helm rollback documentdb-operator --namespace documentdb-system
```

### 2. Image Rollback
```yaml
# Update DocumentDB CR with previous image
spec:
  documentDBImage: "mcr.microsoft.com/documentdb/gateway:v2.0.0"
```

### 3. Database Rollback
```bash
# Restore from backup
kubectl exec -it documentdb-cluster-1 -- psql documentdb < backup.sql
```

## Testing Strategy

### 1. Unit Tests
- Operator upgrade logic
- Image compatibility checks
- CRD validation

### 2. Integration Tests
- End-to-end upgrade scenarios
- Rollback procedures
- Multi-version compatibility

### 3. Performance Tests
- Upgrade impact on performance
- Connection handling during upgrades
- Resource utilization

## Documentation Requirements

### 1. Upgrade Guides
- Step-by-step upgrade procedures
- Version-specific considerations
- Troubleshooting guides

### 2. Release Notes
- Breaking changes
- New features
- Known issues

## Best Practices

### 1. Development
- Maintain backward compatibility
- Use feature flags for new features
- Implement proper logging

### 2. Operations
- Always backup before upgrades
- Test upgrades in staging
- Monitor during upgrades
- Have rollback plan ready

### 3. Communication
- Notify users of upgrade windows
- Provide upgrade instructions
- Document known issues

## Conclusion

The DocumentDB Kubernetes operator uses a **unified versioning strategy** that significantly simplifies upgrade operations while ensuring component compatibility and system reliability.

## Key Benefits of Unified Versioning

### 1. Operational Excellence
- **Single Version Management**: Track one version (DocumentDB operator) instead of managing 5+ component versions
- **Atomic Upgrades**: All components upgrade together, eliminating partial upgrade states
- **Simplified Rollbacks**: Single-command rollback affects all components consistently
- **Reduced Complexity**: No component version compatibility matrix to manage

### 2. Quality Assurance  
- **End-to-End Testing**: All components tested together as a complete system
- **Guaranteed Compatibility**: No integration issues between mismatched component versions
- **Predictable Behavior**: System behavior is deterministic across environments

### 3. Customer Experience
- **Clear Release Communication**: Single version number covers all component updates
- **Simplified Documentation**: One upgrade guide instead of multiple component guides
- **Easier Support**: Support teams only need to track DocumentDB operator version

## Implementation Strategy

### Immediate Benefits
1. **Simplified Operations**: 
   ```bash
   # Before (multiple components to track):
   # operator:v1.2.3, gateway:v2.1.0, postgres:16.2-ext-1.1, injector:v1.0.5, cnpg:v0.24.0
   
   # After (unified versioning):
   helm upgrade documentdb-operator ./chart --version v1.3.0
   # All components automatically aligned to v1.3.0
   ```

2. **Reduced Risk**: Eliminates component version mismatches and integration failures

3. **Faster Releases**: Single CI/CD pipeline builds and tests all components together

### Migration Path
- **New Deployments**: Start with unified versioning from day one
- **Existing Deployments**: Migrate to unified versioning at next major release
- **Legacy Support**: Maintain backward compatibility during transition period

## Success Factors

With unified versioning, the key success factors become:

### 1. Release Engineering
- **Synchronized Builds**: All component images built and tagged together
- **Comprehensive Testing**: Full system testing for every release
- **Clear Versioning**: Semantic versioning applied to entire system

### 2. Operations
- **Single Upgrade Command**: `helm upgrade` handles all components
- **Comprehensive Monitoring**: Monitor all components during upgrade
- **Atomic Rollbacks**: `helm rollback` reverts entire system

### 3. Documentation
- **Unified Release Notes**: Single document covering all component changes
- **Clear Upgrade Paths**: Simple version-to-version upgrade documentation
- **Centralized Support**: Single version for troubleshooting

The unified versioning approach transforms DocumentDB operator upgrades from a complex multi-component orchestration into a simple, reliable, single-command operation while maintaining the highest standards for data safety and system reliability.

