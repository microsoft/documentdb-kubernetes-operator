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

### 1. DocumentDB Operator Upgrade (Helm Chart)
- **Trigger**: New operator version release
- **Scope**: Control plane components
- **Impact**: Low (no data plane disruption)

### 2. Sidecar Injector Upgrade
- **Trigger**: Updates to injection logic
- **Scope**: Control plane webhook
- **Impact**: Medium (affects new pod creation)

### 3. Gateway Image Upgrade
- **Trigger**: New gateway version with features/fixes
- **Scope**: Data plane (requires pod restart)
- **Impact**: Medium (rolling restart of pods)
- **State**: **Stateless** - Gateway containers have no persistent state
- **Risk**: Low - No data loss risk, only temporary connection disruption

### 4. CNPG Operator Upgrade
- **Trigger**: Upstream CNPG operator updates
- **Scope**: Control plane and data plane
- **Impact**: Variable (depends on CNPG upgrade requirements)

### 5. PostgreSQL Database Upgrade
- **Trigger**: PostgreSQL version bump (e.g., 14.x → 15.x or 14.2 → 14.3)
- **Scope**: Data plane (requires careful database migration)
- **Impact**: High (potential data migration and downtime required)
- **State**: **Stateful** - PostgreSQL contains persistent application data
- **Risk**: High - Data migration required, potential for data corruption or loss
- **Categories**:
  - **Minor Version**: 14.2 → 14.3 (in-place upgrade, low risk)
  - **Major Version**: 14.x → 15.x (migration required, high risk)

### 6. DocumentDB Postgres Extension Upgrade
- **Trigger**: DocumentDB extension updates (new features, bug fixes, compatibility)
- **Scope**: Data plane (requires extension update within PostgreSQL)
- **Impact**: Medium to High (depends on extension changes)
- **State**: **Stateful** - Extension may modify schema or data structures
- **Risk**: Medium to High - Extension schema changes may affect data
- **Categories**:
  - **Patch Updates**: Bug fixes, minor improvements (medium risk)
  - **Feature Updates**: New DocumentDB features, API changes (high risk)
  - **Breaking Changes**: Schema modifications, compatibility breaks (very high risk)

## Upgrade Strategies

### 1. DocumentDB Operator Upgrade Strategy

The DocumentDB operator upgrade focuses on the core operator deployment and Custom Resource Definitions (CRDs). This involves upgrading the operator's control plane components and handling CRD version migrations while maintaining backward compatibility.

#### A. Operator Components

**Core Operator Deployment:**
- **Controller Manager**: Main operator logic and reconciliation loops
- **Webhook Server**: Admission controllers and CRD conversion webhooks  
- **RBAC Resources**: Service accounts, roles, and bindings
- **Custom Resource Definitions**: DocumentDB CRDs with version support

**Note**: CNPG operator dependency management is handled separately. See [Section 4: CNPG Operator Upgrade Strategy](#4-cnpg-operator-upgrade-strategy) for details on CNPG version coupling and upgrade procedures.

#### B. Upgrade Strategy

**Rolling Upgrade (Recommended for all DocumentDB operator versions)**

Since the DocumentDB operator manages control plane components (CRDs, controllers, webhooks, RBAC) and does not directly handle stateful data, rolling upgrades are the appropriate strategy for all operator versions.

```bash
# Step 1: Pre-upgrade validation
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --dry-run \
  --debug

# Step 2: Perform rolling upgrade
helm upgrade documentdb-operator ./documentdb-chart \
  --namespace documentdb-system \
  --wait \
  --timeout 600s \
  --atomic

# Step 3: Verify operator health
kubectl rollout status deployment/documentdb-operator -n documentdb-system
```

**Why Rolling Upgrade is Sufficient:**
- **Stateless Control Plane**: Operator controllers, webhooks, and RBAC are stateless
- **CRD Compatibility**: CRD versioning and conversion webhooks handle schema evolution
- **Zero Data Impact**: Operator upgrades don't affect PostgreSQL data or running clusters
- **Fast Rollback**: Helm rollback is simple for control plane components

**Note**: For stateful components (PostgreSQL data migration), see [Section 5: PostgreSQL Database Upgrade Strategy](#5-postgresql-database-upgrade-strategy) which covers blue-green deployments for database upgrades.

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

#### D. Validation and Troubleshooting

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

**Troubleshooting Common Issues:**

1. **Injector webhook not responding:**
```bash
# Check injector pod status
kubectl get pods -l app=sidecar-injector -n cnpg-system

# Check webhook configuration
kubectl describe mutatingwebhookconfiguration documentdb-sidecar-injector

# Verify TLS certificates
kubectl get secret sidecar-injector-certs -n cnpg-system -o yaml
```

2. **Pod recreation failed:**
```bash
# Check CNPG cluster status
kubectl get clusters.postgresql.cnpg.io -A -o wide

# Check pod events for recreation issues
kubectl describe pods -l cnpg.io/cluster

# Manual pod deletion if annotation-based restart failed
kubectl delete pods -l cnpg.io/cluster=<cluster-name> -n <namespace>
```

3. **Injection not applied to recreated pods:**
```bash
# Verify injector is targeting correct pods
kubectl get mutatingwebhookconfiguration documentdb-sidecar-injector -o jsonpath='{.webhooks[0].namespaceSelector}'

# Check if pods have required labels/annotations for injection
kubectl describe pods -l cnpg.io/cluster | grep -E "(Labels|Annotations)"
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

##### Minor Version Upgrades (e.g., 14.2 → 14.3)
**Risk Level**: Low to Medium
**Strategy**: In-place upgrade with rolling restart

```yaml
# For minor PostgreSQL versions
spec:
  postgresql:
    parameters:
      shared_preload_libraries: "documentdb_extension"
    image: "mcr.microsoft.com/documentdb/postgres:14.3-ext-1.1"
```

**Process:**
1. Create automated backup before upgrade
2. Update image in DocumentDB CR
3. CNPG performs rolling restart
4. Validate data integrity post-upgrade

##### Major Version Upgrades (e.g., 14.x → 15.x)
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
  imageName: "mcr.microsoft.com/documentdb/postgres:14.3-ext-1.1"
  # CNPG will handle the upgrade process
```

##### Strategy 2: Blue-Green Migration (Major Versions and High-Risk Upgrades)

**When to Use Blue-Green for PostgreSQL:**
- **Major PostgreSQL versions** (e.g., 14.x → 15.x)
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
  postgresqlImage: "mcr.microsoft.com/documentdb/postgres:15.2-ext-1.2"  # New version
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
  postgresqlImage: "mcr.microsoft.com/documentdb/postgres:14.x-ext-2.0"
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

