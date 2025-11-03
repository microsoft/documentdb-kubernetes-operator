#!/bin/bash
VERSION=1
helm dependency update documentdb-chart
# Set image registry and tag if provided
if [ -n "$IMAGE_REGISTRY" ]; then
    echo "Setting image registry to: $IMAGE_REGISTRY"
    # Update values.yaml or use --set flags during install
    HELM_SET_ARGS="--set image.documentdbk8soperator.repository=$IMAGE_REGISTRY/operator"
    HELM_SET_ARGS="$HELM_SET_ARGS --set image.sidecarinjector.repository=$IMAGE_REGISTRY/sidecar-injector"
fi

# Package the Helm chart
echo "Packaging Helm chart with version 0.0.${VERSION}..."
helm package documentdb-chart --version 0.0.${VERSION}

# Define chart name
CHART_NAME="documentdb-operator"

# Define namespace
NAMESPACE="documentdb-operator"

# Install the Helm chart
echo "Installing $CHART_NAME operator..."
helm install $CHART_NAME ./documentdb-operator-0.0.${VERSION}.tgz \
    --namespace $NAMESPACE \
    --create-namespace \
    $HELM_SET_ARGS \
    --namespace $NAMESPACE \
    --create-namespace \
    $HELM_SET_ARGS
