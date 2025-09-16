#!/bin/bash

# DocumentDB Operator Upgrade Script
# This script upgrades the DocumentDB operator to a new version

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
    echo "export OPERATOR_VERSION=0.1.2"
    exit 1
fi

echo "🚀 Starting DocumentDB Operator Upgrade"
echo "📦 New Version: ${OPERATOR_VERSION}"
echo "🏗️  Registry: ${REPO_NAME}.azurecr.io"
echo ""

# Option to retag instead of rebuild
if [[ "$1" == "--retag" ]]; then
    echo "🏷️  Using retag mode (no rebuilding)"
    RETAG_MODE=true
    if [[ -z "$2" ]]; then
        echo "❌ Error: --retag mode requires source version"
        echo "Usage: $0 --retag <source_version>"
        echo "Example: $0 --retag 0.1.1"
        exit 1
    fi
    SOURCE_VERSION=$2
else
    RETAG_MODE=false
fi

# Step 1: Login to ACR
echo "🔐 Logging into Azure Container Registry..."
az acr login --name ${REPO_NAME}

# Step 2: Build/retag and push images
if [[ "$RETAG_MODE" == "true" ]]; then
    echo "🏷️  Retagging existing images from ${SOURCE_VERSION} to ${OPERATOR_VERSION}..."
    docker tag ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${SOURCE_VERSION} ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}
    docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}
    
    docker tag ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${SOURCE_VERSION} ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
    docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
else
    echo "🔨 Building and pushing new operator image..."
    docker build -t ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION} .
    docker push ${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE}:${OPERATOR_VERSION}

    echo "🔨 Building and pushing new sidecar injector..."
    cd plugins/sidecar-injector/
    docker build -t ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION} .
    docker push ${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE}:${OPERATOR_VERSION}
    cd ../..
fi

# Step 3: Package new Helm chart
echo "📦 Packaging new Helm chart..."
helm package documentdb-chart --version ${OPERATOR_VERSION}

# Step 4: Upgrade operator
echo "⬆️  Upgrading DocumentDB operator..."
helm upgrade documentdb-operator ./documentdb-operator-${OPERATOR_VERSION}.tgz \
  --namespace documentdb-operator \
  --set image.documentdbk8soperator.repository=${REPO_NAME}.azurecr.io/${OPERATOR_IMAGE} \
  --set image.documentdbk8soperator.tag=${OPERATOR_VERSION} \
  --set image.sidecarinjector.repository=${REPO_NAME}.azurecr.io/${SIDECAR_INJECTOR_IMAGE} \
  --set image.sidecarinjector.tag=${OPERATOR_VERSION}

# Step 5: Cleanup
echo "🧹 Cleaning up temporary files..."
rm -rf documentdb-operator-${OPERATOR_VERSION}.tgz

# Step 6: Verify upgrade
echo "✅ Verifying upgrade..."
echo ""
echo "🔄 Operator pods (should be restarting):"
kubectl get pods -n documentdb-operator
echo ""
echo "🔄 CNP-G system pods (should be restarting):"
kubectl get pods -n cnpg-system
echo ""
echo "📸 New operator image:"
kubectl get deployment documentdb-operator -n documentdb-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
echo ""
echo ""

echo "🎉 DocumentDB Operator upgrade completed!"
echo ""
echo "📋 Next steps:"
echo "1. Check if DocumentDB clusters remain unchanged: kubectl get pods -n documentdb-test-ns"