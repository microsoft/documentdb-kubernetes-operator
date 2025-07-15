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

### 1. DocumentDB Operator
- **Component**: Custom Kubernetes operator (Go-based)
- **Distribution**: Helm chart
- **Function**: Manages DocumentDB custom resources and orchestrates CNPG clusters
- **Versioning**: Semantic versioning (e.g., v1.2.3)

### 2. Gateway Container
- **Component**: DocumentDB Gateway (MongoDB protocol translator)
- **Distribution**: Docker image
- **Function**: Translates MongoDB protocol to PostgreSQL queries
- **Versioning**: Semantic versioning aligned with operator

### 3. PostgreSQL with DocumentDB Extension
- **Component**: PostgreSQL server with DocumentDB extensions
- **Distribution**: Docker image
- **Function**: Data storage and processing layer
- **Versioning**: PostgreSQL version + DocumentDB extension version

### 4. CNPG Sidecar Injector
- **Component**: Sidecar injection webhook
- **Distribution**: Docker image
- **Function**: Injects Gateway sidecar into CNPG pods
- **Versioning**: Semantic versioning aligned with operator

### 5. CNPG Operator (Dependency)
- **Component**: CloudNative-PG operator
- **Distribution**: Helm chart (as dependency)
- **Function**: PostgreSQL cluster management
- **Versioning**: Independent upstream versioning

## Upgrade Scenarios

### 1. Operator Upgrade (Helm Chart)
- **Trigger**: New operator version release
- **Scope**: Control plane components
- **Impact**: Low (no data plane disruption)

### 2. Gateway Image Upgrade
- **Trigger**: New gateway version with features/fixes
- **Scope**: Data plane (requires pod restart)
- **Impact**: Medium (rolling restart of pods)
- **State**: **Stateless** - Gateway containers have no persistent state
- **Risk**: Low - No data loss risk, only temporary connection disruption

### 3. PostgreSQL + DocumentDB Extension Upgrade
- **Trigger**: PostgreSQL version bump or extension updates
- **Scope**: Data plane (requires careful migration)
- **Impact**: High (potential data migration required)
- **State**: **Stateful** - PostgreSQL contains persistent application data
- **Risk**: High - Data migration required, potential for data corruption or loss

### 4. CNPG Operator Upgrade
- **Trigger**: Upstream CNPG operator updates
- **Scope**: Control plane and data plane
- **Impact**: Variable (depends on CNPG upgrade requirements)

### 5. Sidecar Injector Upgrade
- **Trigger**: Updates to injection logic
- **Scope**: Control plane webhook
- **Impact**: Medium (affects new pod creation)

## Upgrade Strategies

### 1. DocumentDB Operator Upgrade Strategy

The DocumentDB operator upgrade is the most critical component as it orchestrates the entire system. Given its dependency on CNPG, we need to ensure seamless integration and compatibility.

#### A. CNPG Dependency Management

**CNPG as Helm Dependency**
```yaml
# documentdb-chart/Chart.yaml
apiVersion: v2
name: documentdb-operator
version: 1.2.0
appVersion: 1.2.0
dependencies:
  - name: cloudnative-pg
    version: "0.24.0"
    repository: https://cloudnative-pg.github.io/charts
    condition: cnpg.enabled
```

**CNPG Version Compatibility Matrix**
| DocumentDB Operator | CNPG Version | Status |
|---------------------|-------------|---------|
| v1.2.x | 0.24.x | Current |
| v1.3.x | 0.26.x | Planned |

**Important**: Each DocumentDB operator version is locked to a specific CNPG version. Customers cannot upgrade CNPG independently.

#### B. Upgrade Strategies

##### Strategy 1: Rolling Upgrade (Recommended for minor versions)
```bash
# Step 1: Update CNPG dependency first
helm dependency update ./documentdb-chart

# Step 2: Upgrade with dry-run validation
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --dry-run \
  --debug

# Step 3: Perform actual upgrade
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --wait \
  --timeout 600s \
  --atomic
```

##### Strategy 2: Blue-Green Upgrade (For major versions)

**Important**: Blue-Green upgrades for DocumentDB require careful data migration since PostgreSQL is stateful.

#### Understanding Blue-Green Deployment for DocumentDB

**Why Separate Namespaces?**
- **Operator Version Isolation**: Different operator versions may have incompatible CRDs or controllers
- **Resource Isolation**: Prevents resource conflicts between old and new operators
- **Safe Rollback**: Blue environment remains intact for immediate rollback if needed
- **Testing Environment**: Green environment can be fully validated before traffic switch

**Data Storage Transfer Requirements:**
- **New PVCs Required**: Green cluster cannot share PVCs with blue cluster due to:
  - Different PostgreSQL versions may have incompatible data formats
  - CNPG cluster names must be unique (blue: `my-documentdb-cluster`, green: `my-documentdb-green-cluster`)
  - PVC naming is tied to StatefulSet names, which are derived from cluster names
- **Data Migration Methods**: pg_dump/restore or logical replication for data transfer
- **Storage Sizing**: Green cluster should have equal or larger storage capacity

#### Step-by-Step Blue-Green Upgrade Process

```bash
# Step 1: Create new namespace for green environment
kubectl create namespace documentdb-system-v2

# Step 2: Deploy new operator version in green namespace
helm install documentdb-operator-v2 ./documentdb-chart \
  --namespace documentdb-system-v2 \
  --set cnpg.enabled=true \
  --set cnpg.version="0.26.0"  # New CNPG version included

# Step 3: Deploy new DocumentDB cluster (Green) with same configuration
kubectl apply -f - <<EOF
apiVersion: db.microsoft.com/v2
kind: DocumentDB
metadata:
  name: my-documentdb-green
  namespace: default
spec:
  # Same configuration as blue cluster
  nodeCount: 1
  instancesPerNode: 1
  documentDBImage: "mcr.microsoft.com/documentdb/gateway:v2.1.0"
  resource:
    pvcSize: "10Gi"  # Same or larger storage size
  exposeViaService:
    serviceType: "ClusterIP"  # Internal during migration
  # Green cluster gets new PVCs automatically
EOF

# Step 4: Wait for green cluster to be ready
kubectl wait --for=condition=Ready documentdb/my-documentdb-green --timeout=600s

# Step 5: Verify green cluster PostgreSQL is accessible
kubectl exec -it my-documentdb-green-cluster-1 -- psql -c "SELECT version();"

# Step 6: Stop writes to blue cluster (maintenance mode)
kubectl patch documentdb my-documentdb -p '{"spec":{"maintenance": true}}'

# Step 7: Perform data migration from blue to green
echo "Starting data migration from blue to green cluster..."

# TODO: Follow the detailed backup/restore guide for complete migration procedures
# Reference: docs/designs/backup-restore/backup-restore-guide.md

# Basic migration example (see backup/restore guide for production procedures)
kubectl exec -it my-documentdb-cluster-1 -- pg_dumpall --clean > final-migration-backup.sql
kubectl exec -i my-documentdb-green-cluster-1 -- psql < final-migration-backup.sql

# Step 8: Validate data integrity on green cluster
kubectl exec -it my-documentdb-green-cluster-1 -- psql -c "
  SELECT 
    schemaname,
    tablename,
    n_tup_ins as row_count
  FROM pg_stat_user_tables 
  WHERE schemaname NOT IN ('information_schema', 'pg_catalog');
"

# Step 9: Compare data between blue and green clusters
BLUE_ROW_COUNT=$(kubectl exec -it my-documentdb-cluster-1 -- psql -t -c "SELECT COUNT(*) FROM pg_stat_user_tables;")
GREEN_ROW_COUNT=$(kubectl exec -it my-documentdb-green-cluster-1 -- psql -t -c "SELECT COUNT(*) FROM pg_stat_user_tables;")

echo "Blue cluster tables: $BLUE_ROW_COUNT"
echo "Green cluster tables: $GREEN_ROW_COUNT"

# Step 10: Test MongoDB connectivity to green cluster
mongosh "mongodb://username:password@my-documentdb-green-service:27017/test" --eval "db.test.findOne()"

# Step 11: Switch traffic from blue to green
kubectl patch service my-documentdb-service -p '{
  "spec": {
    "selector": {
      "cnpg.io/cluster": "my-documentdb-green"
    }
  }
}'

# Step 12: Validate green cluster is serving traffic
mongosh "mongodb://username:password@my-documentdb-service:27017/test" --eval "db.test.findOne()"

# Step 13: Monitor green cluster for 24-48 hours before cleanup
echo "Green cluster is now serving traffic. Monitor for 24-48 hours before cleanup."
echo "Blue cluster preserved for rollback: my-documentdb-cluster"
echo "Blue namespace preserved for rollback: documentdb-system"

# Step 14: Cleanup blue cluster after validation period (run after 24-48 hours)
# kubectl delete documentdb my-documentdb
# kubectl delete namespace documentdb-system  # Old operator version
```

**Important**: For detailed data migration procedures, storage verification, and advanced backup/restore strategies, refer to the dedicated backup/restore guide: `docs/designs/backup-restore/backup-restore-guide.md`

#### Rollback Procedure

**Immediate Rollback (if issues detected):**
```bash
# Switch traffic back to blue cluster
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
```

**Complete Environment Rollback:**
```bash
# Clean up green environment
kubectl delete documentdb my-documentdb-green
kubectl delete namespace documentdb-system-v2

# Restore blue environment to full operation
kubectl patch documentdb my-documentdb -p '{"spec":{"maintenance": false}}'
```

#### C. CRD Upgrade Handling

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


### 2. Gateway Image Upgrade Strategy

The Gateway container is **stateless** and acts as a protocol translator between MongoDB clients and PostgreSQL. This stateless nature makes upgrades simpler and safer compared to stateful components.

TODO: Detailed Plan

### 3. PostgreSQL + DocumentDB Extension Upgrade Strategy

PostgreSQL is **stateful** and contains all persistent application data. This requires careful planning, extensive testing, and robust backup strategies.

#### A. Stateful Upgrade Challenges
- **Data Persistence**: All application data stored in PostgreSQL
- **Migration Complexity**: Schema and data migration between versions
- **Downtime Risk**: Major upgrades may require downtime
- **Rollback Complexity**: Point-in-time recovery required for rollbacks
- **Validation Requirements**: Extensive data integrity validation needed

#### B. Upgrade Categories

##### Minor Version Upgrades (e.g., 14.2 → 14.3)
TODO:

##### Major Version Upgrades (e.g., 14.x → 15.x)
TODO:

### 4. CNPG Operator Upgrade Strategy

**Important**: CNPG operator upgrades are **not** available as standalone upgrades for customers. The CNPG version is tightly coupled with the DocumentDB operator version to ensure compatibility and stability.

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

**Version Compatibility Check:**
```bash
#!/bin/bash
# validate-cnpg-compatibility.sh

CNPG_VERSION=$1
DOCUMENTDB_VERSION=$(helm list -n documentdb-system -o json | jq -r '.[] | select(.name=="documentdb-operator") | .app_version')

echo "Validating CNPG $CNPG_VERSION compatibility with DocumentDB $DOCUMENTDB_VERSION"

# Check CRD compatibility
kubectl get crd clusters.postgresql.cnpg.io -o jsonpath='{.spec.versions[*].name}'

# Validate API version compatibility
case $CNPG_VERSION in
    "0.24."*)
        if [[ $DOCUMENTDB_VERSION == "1.2."* ]]; then
            echo "✅ Compatible"
        else
            echo "❌ Incompatible - DocumentDB $DOCUMENTDB_VERSION requires CNPG 0.24.x"
            exit 1
        fi
        ;;
    "0.26."*)
        if [[ $DOCUMENTDB_VERSION == "1.3."* ]]; then
            echo "✅ Compatible"
        else
            echo "❌ Incompatible - DocumentDB $DOCUMENTDB_VERSION not compatible with CNPG 0.26.x"
            exit 1
        fi
        ;;
    *)
        echo "❌ Unknown CNPG version compatibility"
        exit 1
        ;;
esac
```

CNPG Operator Upgrade Strategy: TODO

### 5. Sidecar Injector Upgrade Strategy

The DocumentDB Sidecar Injector is a **stateless** webhook that automatically injects the DocumentDB Gateway container into CNPG PostgreSQL pods. It runs as a CNPG plugin and requires coordinated updates with both the injector image and the gateway image it injects.

#### A. Sidecar Injector Architecture

**Component Overview:**
- **Injector Service**: CNPG plugin service running on port 9090
- **Deployment**: Single replica deployment in `cnpg-system` namespace
- **TLS Certificates**: Mutual TLS between injector and CNPG operator
- **Gateway Image**: The DocumentDB Gateway image that gets injected into pods

**Key Dependencies:**
```yaml
# From values.yaml
image:
  sidecarinjector:
    repository: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector
    tag: "001"  # Sidecar injector version
```

#### B. Upgrade Scenarios

##### Scenario 1: Sidecar Injector Image Update
**Trigger**: New injector logic, bug fixes, or configuration changes
**Impact**: Affects new pod creation only (existing pods unchanged)
**Risk**: Medium - Injection failures affect new PostgreSQL pods

##### Scenario 2: Gateway Image Update via Injector
**Trigger**: New gateway version with features/fixes
**Impact**: Affects both injector configuration and injected gateway image
**Risk**: Medium - Existing pods need restart to get new gateway image

#### C. Upgrade Strategy

**Rolling Update (Recommended for All Scenarios)**

Since both the sidecar injector and gateway are **stateless** components, rolling updates provide the optimal balance of safety and simplicity. The stateless nature means:

```bash
# Step 1: Update sidecar injector image in values.yaml
cat <<EOF > values-update.yaml
image:
  sidecarinjector:
    repository: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-sidecar-injector
    tag: "002"  # New version
EOF

# Step 2: Upgrade via Helm (Rolling Update)
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --values values-update.yaml \
  --wait \
  --timeout 300s

# Step 3: Verify injector deployment rollout
kubectl rollout status deployment/sidecar-injector -n cnpg-system

# Step 4: Test injection on new pods (existing pods unaffected)
kubectl delete pod -l cnpg.io/cluster=my-documentdb-cluster --wait=false
kubectl wait --for=condition=Ready pod -l cnpg.io/cluster=my-documentdb-cluster --timeout=300s

# Step 5: Verify new gateway image is injected
kubectl get pods -l cnpg.io/cluster=my-documentdb-cluster -o jsonpath='{.items[0].spec.containers[?(@.name=="gateway")].image}'
```


## Upgrade Orchestration

### 1. Upgrade Order (Recommended)

```mermaid
graph TD
    A[CNPG Operator] --> B[DocumentDB Operator]
    B --> C[Sidecar Injector]
    C --> D[Gateway Image]
    D --> E[PostgreSQL + Extension]
```

### 2. Compatibility Matrix

| Component | Version | Gateway | PostgreSQL | CNPG | Operator | Notes |
|-----------|---------|---------|------------|------|----------|-------|
| Gateway | v2.1.0 | ✓ | 14.x | 0.24.x | v1.2.0 | Stateless |
| PostgreSQL | 14.2 | v2.1.0+ | ✓ | 0.24.x | v1.2.0 | Stateful |
| CNPG | 0.24.0 | v2.1.0+ | 14.x+ | ✓ | v1.2.0 | Locked to operator version |
| Operator | v1.2.0 | v2.1.0+ | 14.x+ | 0.24.x | ✓ | Controls CNPG version |

**Important**: CNPG version is locked to the DocumentDB operator version. Customers upgrade CNPG only through DocumentDB operator upgrades.

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

The DocumentDB Kubernetes operator upgrade strategy must account for multiple interdependent components while ensuring minimal downtime and data integrity. The recommended approach prioritizes safety through staged rollouts, comprehensive testing, and robust rollback procedures.

Key success factors:
- Proper component versioning and compatibility matrix
- Comprehensive testing at each stage
- Clear documentation and communication
- Automated validation and rollback procedures

