#!/bin/bash

# DocumentDB Operator Installation Script
# This script builds, packages, and installs the DocumentDB operator on AKS

set -e

# Check if required variables are set
if [[ -z "$REPO_NAME" || -z "$OPERATOR_IMAGE" || -z "$SIDECAR_INJECTOR_IMAGE" || -z "$OPERATOR_VERSION" ]]; then
    echo "❌ Error: Required environment variables not set"
    echo "Please set: REPO_NAME, OPERATOR_IMAGE, SIDECAR_INJECTOR_IMAGE, OPERATOR_VERSION"
    echo ""
    echo "Example:"
    echo "export REPO_NAME=pgcosmoscontroller"
    echo "export OPERATOR_IMAGE=documentdb-k8s-operator"
    echo "export SIDECAR_INJECTOR_IMAGE=cnpg-plugin"
    echo "export OPERATOR_VERSION=0.1.1"
    exit 1
fi

echo "🚀 Starting DocumentDB Operator Installation"
echo "📦 Version: ${OPERATOR_VERSION}"
echo "🏗️  Registry: ${REPO_NAME}.azurecr.io"
echo ""

# Step 1: Login to ACR
echo "🔐 Logging into Azure Container Registry..."
az acr login --name ${REPO_NAME}

# Step 2: Build and push operator image
echo "🔨 Building and pushing operator image..."
docker build -t ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}

# Step 3: Build and push sidecar injector
echo "🔨 Building and pushing sidecar injector..."
cd plugins/sidecar-injector/
go build -o bin/cnpg-i-sidecar-injector main.go
docker build -t ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION} .
docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
cd ../..

# Step 4: Package Helm chart
echo "📦 Packaging Helm chart..."
helm dependency update documentdb-chart
helm package documentdb-chart --version ${OPERATOR_VERSION}

# Step 5: Check for conflicts
echo "🔍 Checking for namespace conflicts..."
kubectl get namespace cnpg-system 2>/dev/null && echo "⚠️  cnpg-system exists, may need cleanup" || echo "✅ cnpg-system doesn't exist"

# Step 6: Install operator
echo "⚙️  Installing DocumentDB operator..."
helm install documentdb-operator ./documentdb-operator-${OPERATOR_VERSION}.tgz \
  --namespace documentdb-operator --create-namespace \
  --set image.documentdbk8soperator.repository=${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE} \
  --set image.documentdbk8soperator.tag=${OPERATOR_VERSION} \
  --set image.sidecarinjector.repository=${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE} \
  --set image.sidecarinjector.tag=${OPERATOR_VERSION}

# Step 7: Cleanup
echo "🧹 Cleaning up temporary files..."
rm -rf documentdb-operator-${OPERATOR_VERSION}.tgz

# Step 8: Verify installation
echo "✅ Verifying installation..."
echo ""
kubectl get pods -A | grep -E "(documentdb-operator|cnpg-system)" || echo "⚠️  No pods found - may still be starting"

echo ""
echo "🎉 DocumentDB Operator installation completed!"
echo "🔍 Run 'kubectl get pods -A | grep -E \"(documentdb-operator|cnpg-system)\"' to check pod status"