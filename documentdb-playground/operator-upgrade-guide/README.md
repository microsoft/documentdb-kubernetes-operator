# DocumentDB Operator Upgrade Testing - AKS

**Local Development Guide** for testing DocumentDB operator control plane upgrades with custom images and charts.

## 📋 Guide Scope

This guide is designed for **local development and testing scenarios** where you:
- Build custom operator images locally 
- Package and test new Helm chart versions
- Test upgrade mechanics and version management
- Validate operator behavior during control plane upgrades

### 🏭 Production Deployments

For **production upgrades** with existing published images/charts, the process is much simpler:

```bash
# Simple production upgrade (existing images)
helm upgrade documentdb-operator <chart-repo>/documentdb-operator \
  --version <new-chart-version> \
  --namespace documentdb-operator
```

This guide focuses on the **development workflow** where you build and test everything locally.

## Configuration

Set these variables for your environment:

```bash
REPO_NAME=pgcosmoscontroller
OPERATOR_IMAGE=documentdb-k8s-operator
SIDECAR_INJECTOR_IMAGE=cnpg-plugin
OPERATOR_VERSION=0.1.1
```

## Quick AKS Workflow

### Build and Push Images

```bash
# Login to ACR
az acr login --name ${REPO_NAME}

# Build and push operator
docker build -t ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}

# Build and push plugin (if needed)
cd plugins/sidecar-injector/
go build -o bin/cnpg-i-sidecar-injector main.go
docker build -t ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
```

### Helm Package and Install

```bash
# Go back to root directory
cd ../..

# Update helm dependencies
helm dependency update documentdb-chart

# Package chart
helm package documentdb-chart --version ${OPERATOR_VERSION}

# Check and handle cnpg-system namespace conflict (if needed)
kubectl get namespace cnpg-system 2>/dev/null && echo "cnpg-system exists, may need cleanup" || echo "cnpg-system doesn't exist"

# Clean up CloudNative-PG resources with conflicting ownership metadata
# (These were likely installed previously in 'default' namespace context)
# kubectl delete namespace cnpg-system 2>/dev/null || true
# kubectl delete clusterrole documentdb-operator-cloudnative-pg 2>/dev/null || true
# kubectl delete clusterrole documentdb-operator-cloudnative-pg-view 2>/dev/null || true
# kubectl delete clusterrole documentdb-operator-cloudnative-pg-edit 2>/dev/null || true
# kubectl delete clusterrolebinding documentdb-operator-cloudnative-pg 2>/dev/null || true
# kubectl delete clusterrole documentdb-operator-cluster-role 2>/dev/null || true
# kubectl delete clusterrolebinding documentdb-operator-cluster-rolebinding 2>/dev/null || true
# kubectl delete mutatingwebhookconfiguration cnpg-mutating-webhook-configuration 2>/dev/null || true
# kubectl delete validatingwebhookconfiguration cnpg-validating-webhook-configuration 2>/dev/null || true

# Install operator with custom image parameters
helm install documentdb-operator ./documentdb-operator-${OPERATOR_VERSION}.tgz \
  --namespace documentdb-operator --create-namespace \
  --set image.documentdbk8soperator.repository=${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE} \
  --set image.documentdbk8soperator.tag=${OPERATOR_VERSION} \
  --set image.sidecarinjector.repository=${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE} \
  --set image.sidecarinjector.tag=${OPERATOR_VERSION}

# Alternative: Install with custom values file
# helm install documentdb-operator ./documentdb-operator-${OPERATOR_VERSION}.tgz \
#   --namespace documentdb-operator --create-namespace -f custom-values.yaml

# Cleanup when needed
# helm uninstall documentdb-operator
rm -rf documentdb-operator-${OPERATOR_VERSION}.tgz
```

### Operator Verification

After successful installation, verify all pods are running:

```bash
kubectl get pods -A | grep -E "(documentdb-operator|cnpg-system)"
```

You should see three pods running:
```
NAMESPACE             NAME                                                  READY   STATUS    RESTARTS   AGE
cnpg-system           documentdb-operator-cloudnative-pg-765cc6fc9c-vh2bv   1/1     Running   0          31s
cnpg-system           sidecar-injector-65549f7547-hxlpw                     1/1     Running   0          31s
documentdb-operator   documentdb-operator-d9f556b5-vrhs9                    1/1     Running   0          31s
```

**Expected pods:**
- **documentdb-operator**: Main DocumentDB operator in `documentdb-operator` namespace
- **documentdb-operator-cloudnative-pg**: CloudNative-PG controller in `cnpg-system` namespace  
- **sidecar-injector**: DocumentDB sidecar injector plugin in `cnpg-system` namespace

## Testing Operator Upgrade

### Step 1: Deploy a DocumentDB Cluster

Deploy the test DocumentDB cluster using the existing test file:

```bash
# Deploy the test cluster (includes namespace and credentials)
kubectl apply -f documentdb-playground/operator-upgrade-guide/test-documentdb-cluster.yaml
```

The test cluster is configured without explicit image versions, so it will use the operator's environment defaults.

### Step 2: Verify DocumentDB Cluster is Running

```bash
# Wait for cluster to be ready
kubectl get documentdb -n documentdb-test-ns -w

# Check pods (should see PostgreSQL cluster pods)
kubectl get pods -n documentdb-test-ns

# Get current image versions (baseline)
kubectl get pods -n documentdb-test-ns -o jsonpath='{range .items[*]}{.metadata.name}{": "}{range .spec.containers[*]}{.image}{" "}{end}{"\n"}{end}'
```

### Step 3: Upgrade Operator to New Version

```bash
# Update version for upgrade
OPERATOR_VERSION=0.1.2

# Option A: Build and push new operator version
docker build -t ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}

# Build and push new plugin version  
cd plugins/sidecar-injector/
docker build -t ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
cd ../..

# Option B: If no code changes, you can retag existing images instead:
# docker tag ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:0.1.1 ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}
# docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}
# docker tag ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:0.1.1 ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
# docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}

# Package new chart version
helm package documentdb-chart --version ${OPERATOR_VERSION}

# Upgrade operator using helm upgrade
helm upgrade documentdb-operator ./documentdb-operator-${OPERATOR_VERSION}.tgz \
  --namespace documentdb-operator \
  --set image.documentdbk8soperator.repository=${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE} \
  --set image.documentdbk8soperator.tag=${OPERATOR_VERSION} \
  --set image.sidecarinjector.repository=${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE} \
  --set image.sidecarinjector.tag=${OPERATOR_VERSION}
```

### Step 4: Verify Operator Upgrade

```bash
# Check operator pods restarted with new version
kubectl get pods -n documentdb-operator
kubectl get pods -n cnpg-system

# Verify operator is using new image version
kubectl get deployment documentdb-operator -n documentdb-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
```

### Step 5: Verify DocumentDB Cluster Behavior

```bash
# Check if DocumentDB cluster pods changed (they should NOT change automatically)
kubectl get pods -n documentdb-test-ns

# Compare image versions (should be same as baseline)
kubectl get pods -n documentdb-test-ns -o jsonpath='{range .items[*]}{.metadata.name}{": "}{range .spec.containers[*]}{.image}{" "}{end}{"\n"}{end}'
```

### Expected Behavior

- ✅ **Operator pods restart** with new image versions (0.1.2)
- ✅ **DocumentDB cluster pods remain unchanged** initially 
- ✅ **Cluster updates only when reconciliation is triggered** (manual annotation or spec change)

### Cleanup

```bash
kubectl delete -f documentdb-playground/operator-upgrade-guide/test-documentdb-cluster.yaml
rm -f documentdb-operator-${OPERATOR_VERSION}.tgz
```


## Test Files

- **test-documentdb-cluster.yaml**: Basic DocumentDB cluster for testing
- **test-documentdb-versioned.yaml**: DocumentDB cluster with explicit version pinning
- **install_operator_versioned.sh**: Installation script with version control
- **upgrade_operator.sh**: Upgrade script for version testing
- **operator-install.sh**: Automated installation script (requires environment variables)
- **operator-upgrade.sh**: Automated upgrade script with retag option

## Quick Scripts Usage

For faster workflows, use the automated scripts:

### Installation
```bash
# Set environment variables
export REPO_NAME=pgcosmoscontroller
export OPERATOR_IMAGE=documentdb-k8s-operator
export SIDECAR_INJECTOR_IMAGE=cnpg-plugin
export OPERATOR_VERSION=0.1.1

# Run installation script
./documentdb-playground/operator-upgrade-guide/operator-install.sh
```

### Upgrade
```bash
# Update version
export OPERATOR_VERSION=0.1.2

# Option 1: Full rebuild and upgrade
./documentdb-playground/operator-upgrade-guide/operator-upgrade.sh

# Option 2: Retag existing images (no code changes)
./documentdb-playground/operator-upgrade-guide/operator-upgrade.sh --retag 0.1.1
```

## Troubleshooting

### CloudNative-PG Resource Conflicts

The DocumentDB operator includes CloudNative-PG as a dependency chart. These conflicts occur when CloudNative-PG resources already exist from a previous installation with different Helm ownership metadata.

**Current setup:**
- DocumentDB operator → `documentdb-operator` namespace  
- CloudNative-PG (dependency) → `cnpg-system` namespace

If you get errors like:
```
Error: INSTALLATION FAILED: Unable to continue with install: Namespace "cnpg-system" in namespace "" exists and cannot be imported into the current release: invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-namespace" must equal "documentdb-operator": current value is "default"
```

This means CloudNative-PG was previously installed with metadata pointing to "default" namespace, but now it needs to be managed by the "documentdb-operator" release.

**Solution: Complete cleanup (Recommended):**
```bash
# Delete all conflicting DocumentDB and CloudNative-PG resources
kubectl delete namespace cnpg-system 2>/dev/null || true
kubectl delete namespace documentdb-operator 2>/dev/null || true
kubectl delete clusterrole documentdb-operator-cluster-role 2>/dev/null || true
kubectl delete clusterrole documentdb-operator-cloudnative-pg 2>/dev/null || true
kubectl delete clusterrole documentdb-operator-cloudnative-pg-view 2>/dev/null || true
kubectl delete clusterrole documentdb-operator-cloudnative-pg-edit 2>/dev/null || true
kubectl delete clusterrolebinding documentdb-operator-cluster-rolebinding 2>/dev/null || true
kubectl delete clusterrolebinding documentdb-operator-cloudnative-pg 2>/dev/null || true
kubectl delete mutatingwebhookconfiguration cnpg-mutating-webhook-configuration 2>/dev/null || true
kubectl delete validatingwebhookconfiguration cnpg-validating-webhook-configuration 2>/dev/null || true
kubectl delete crd clusters.postgresql.cnpg.io 2>/dev/null || true
kubectl delete crd backups.postgresql.cnpg.io 2>/dev/null || true
kubectl delete crd scheduledbackups.postgresql.cnpg.io 2>/dev/null || true
kubectl delete crd poolers.postgresql.cnpg.io 2>/dev/null || true
kubectl delete crd dbs.documentdb.io 2>/dev/null || true

# Then retry the helm install command
```