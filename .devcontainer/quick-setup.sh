#!/bin/bash
# DocumentDB Kubernetes Operator Quick Setup Script
# This script helps developers quickly set up a complete local development environment

set -e

CLUSTER_NAME="${1:-documentdb-dev}"
NAMESPACE="${2:-documentdb-test}"

echo "🚀 DocumentDB Kubernetes Operator Quick Setup"
echo "Cluster: $CLUSTER_NAME"
echo "Namespace: $NAMESPACE"
echo ""

# Check if kind cluster exists
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "📦 Creating kind cluster '$CLUSTER_NAME'..."
    if [ -f ".devcontainer/kind-config.yaml" ]; then
        kind create cluster --name "$CLUSTER_NAME" --config .devcontainer/kind-config.yaml
    else
        kind create cluster --name "$CLUSTER_NAME"
    fi
else
    echo "✅ Kind cluster '$CLUSTER_NAME' already exists"
fi

# Set kubectl context
echo "🔧 Setting kubectl context..."
kubectl config use-context "kind-$CLUSTER_NAME"

# Verify cluster is ready
echo "🔍 Verifying cluster is ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=60s

# Install cert-manager (required dependency)
echo "🔐 Installing cert-manager..."
if ! kubectl get namespace cert-manager >/dev/null 2>&1; then
    kubectl create namespace cert-manager
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.yaml
    echo "⏳ Waiting for cert-manager to be ready..."
    kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=120s
else
    echo "✅ cert-manager already installed"
fi

# Build and load operator image
echo "🔨 Building operator image..."
make docker-build IMG=controller:latest

echo "📤 Loading operator image into kind cluster..."
kind load docker-image controller:latest --name "$CLUSTER_NAME"

# Deploy the operator
echo "🚀 Deploying DocumentDB operator..."
make deploy IMG=controller:latest

# Wait for operator to be ready
echo "⏳ Waiting for operator to be ready..."
kubectl wait --for=condition=Available deployment/documentdb-operator-controller-manager -n documentdb-operator-system --timeout=120s

# Create test namespace and credentials
echo "📝 Creating test namespace and credentials..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: documentdb-credentials
  namespace: $NAMESPACE
type: Opaque
stringData:
  username: testuser     
  password: TestPass123
EOF

# Create a sample DocumentDB instance
echo "🗄️ Creating sample DocumentDB instance..."
kubectl apply -f - <<EOF
apiVersion: db.microsoft.com/preview
kind: DocumentDB
metadata:
  name: documentdb-sample
  namespace: $NAMESPACE
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 1Gi
  exposeViaService:
    serviceType: ClusterIP
EOF

echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Useful commands:"
echo "   kubectl get pods -n $NAMESPACE -w              # Watch DocumentDB pods"
echo "   kubectl logs -f deployment/documentdb-operator-controller-manager -n documentdb-operator-system  # Operator logs"
echo "   kubectl get documentdb -n $NAMESPACE           # List DocumentDB instances"
echo "   kubectl describe documentdb documentdb-sample -n $NAMESPACE  # Describe DocumentDB instance"
echo ""
echo "🔌 To connect to DocumentDB once it's ready:"
echo "   kubectl port-forward pod/documentdb-sample-1 10260:10260 -n $NAMESPACE"
echo "   mongosh 127.0.0.1:10260 -u testuser -p TestPass123 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates"
echo ""
echo "🧹 To clean up:"
echo "   kubectl delete documentdb documentdb-sample -n $NAMESPACE"
echo "   kubectl delete namespace $NAMESPACE"
echo "   make undeploy"
echo "   kind delete cluster --name $CLUSTER_NAME"
echo ""