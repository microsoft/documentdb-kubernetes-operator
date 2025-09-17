# Developer Guide

This guide provides comprehensive instructions for setting up and using the development environment for the DocumentDB Kubernetes Operator.

## Table of Contents

- [Quick Start with DevContainer](#quick-start-with-devcontainer)
- [Manual Setup](#manual-setup)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Building and Deploying](#building-and-deploying)
- [Troubleshooting](#troubleshooting)

## Quick Start with DevContainer

The easiest way to get started with development is using the provided DevContainer configuration, which sets up everything you need automatically.

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop) or [Docker Engine](https://docs.docker.com/engine/install/)
- [Visual Studio Code](https://code.visualstudio.com/) with the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

### Getting Started

1. **Clone the repository:**
   ```bash
   git clone https://github.com/microsoft/documentdb-kubernetes-operator.git
   cd documentdb-kubernetes-operator
   ```

2. **Open in DevContainer:**
   - Open the project in VS Code
   - When prompted, click "Reopen in Container" or use the Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`) and select "Dev Containers: Reopen in Container"
   - Wait for the container to build and setup to complete

3. **Verify the setup:**
   ```bash
   # Quick verification of all tools and configuration
   .devcontainer/verify-environment.sh
   
   # Or check individual tools:
   # Check Go installation
   go version
   
   # Check Docker
   docker version
   
   # Check kubectl
   kubectl version --client
   
   # Check kind
   kind version
   
   # Check Helm
   helm version
   ```

4. **Build the operator:**
   ```bash
   make build
   ```

5. **Quick setup with sample deployment:**
   ```bash
   # This script will create a kind cluster, install dependencies, 
   # build and deploy the operator, and create a sample DocumentDB instance
   .devcontainer/quick-setup.sh
   ```

### What's Included in the DevContainer

The DevContainer automatically sets up:

- **Go 1.23** development environment
- **Docker-in-Docker** for building container images
- **kubectl** for Kubernetes cluster interaction
- **Helm** for managing Kubernetes applications
- **kind** for local Kubernetes clusters
- **Development tools**: golangci-lint, goimports, controller-gen, kustomize
- **VS Code extensions**: Go, YAML, Kubernetes tools, and more
- **Pre-configured settings** for Go development

## Manual Setup

If you prefer not to use DevContainer, you can set up the development environment manually.

### Prerequisites

- [Go 1.23+](https://golang.org/doc/install)
- [Docker](https://docs.docker.com/get-docker/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [Helm](https://helm.sh/docs/intro/install/)

### Installation Steps

1. **Clone the repository:**
   ```bash
   git clone https://github.com/microsoft/documentdb-kubernetes-operator.git
   cd documentdb-kubernetes-operator
   ```

2. **Install Go tools:**
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   go install golang.org/x/tools/cmd/goimports@latest
   ```

3. **Install development dependencies:**
   ```bash
   make setup-envtest
   make controller-gen
   make kustomize
   ```

4. **Verify installation:**
   ```bash
   make help
   ```

## Development Workflow

### Setting Up a Local Kubernetes Cluster

1. **Create a kind cluster:**
   ```bash
   kind create cluster --name documentdb-dev
   ```

2. **Verify cluster is running:**
   ```bash
   kubectl cluster-info --context kind-documentdb-dev
   ```

3. **Set kubectl context (if needed):**
   ```bash
   kubectl config use-context kind-documentdb-dev
   ```

### Building the Operator

1. **Build the binary:**
   ```bash
   make build
   ```

2. **Build the Docker image:**
   ```bash
   make docker-build
   ```

3. **Load image into kind cluster:**
   ```bash
   kind load docker-image controller:latest --name documentdb-dev
   ```

### Code Development

1. **Format code:**
   ```bash
   make fmt
   ```

2. **Run static analysis:**
   ```bash
   make vet
   ```

3. **Run linter:**
   ```bash
   make lint
   ```

4. **Fix linting issues automatically:**
   ```bash
   make lint-fix
   ```

5. **Generate manifests and code:**
   ```bash
   make manifests
   make generate
   ```

## Testing

### Unit Tests

Run unit tests to verify your code changes:

```bash
make test
```

### End-to-End Tests

E2E tests require a running kind cluster:

1. **Ensure kind cluster is running:**
   ```bash
   kind get clusters
   ```

2. **Run e2e tests:**
   ```bash
   make test-e2e
   ```

### Manual Testing

For manual testing of the operator:

1. **Deploy the operator:**
   ```bash
   make deploy
   ```

2. **Create a test DocumentDB instance:**
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: Namespace
   metadata:
     name: documentdb-test
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: documentdb-credentials
     namespace: documentdb-test
   type: Opaque
   stringData:
     username: testuser     
     password: TestPass123
   ---
   apiVersion: db.microsoft.com/preview
   kind: DocumentDB
   metadata:
     name: documentdb-test
     namespace: documentdb-test
   spec:
     nodeCount: 1
     instancesPerNode: 1
     documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
     resource:
       pvcSize: 1Gi
     exposeViaService:
       serviceType: ClusterIP
   EOF
   ```

3. **Monitor the deployment:**
   ```bash
   kubectl get pods -n documentdb-test -w
   ```

4. **Check operator logs:**
   ```bash
   kubectl logs -f deployment/documentdb-operator-controller-manager -n documentdb-operator-system
   ```

5. **Clean up:**
   ```bash
   kubectl delete documentdb documentdb-test -n documentdb-test
   kubectl delete namespace documentdb-test
   ```

## Building and Deploying

### Local Development Deploy

1. **Build and load image:**
   ```bash
   make docker-build
   kind load docker-image controller:latest --name documentdb-dev
   ```

2. **Deploy to cluster:**
   ```bash
   make deploy
   ```

3. **Undeploy:**
   ```bash
   make undeploy
   ```

### Building for Production

1. **Build multi-architecture image:**
   ```bash
   make docker-buildx IMG=your-registry/documentdb-operator:tag
   ```

2. **Generate installer manifest:**
   ```bash
   make build-installer
   ```

## Working with Helm Charts

The project includes Helm charts for deployment:

1. **Install cert-manager (required dependency):**
   ```bash
   helm repo add jetstack https://charts.jetstack.io
   helm repo update
   helm install cert-manager jetstack/cert-manager \
     --namespace cert-manager \
     --create-namespace \
     --set installCRDs=true
   ```

2. **Install the operator via Helm:**
   ```bash
   helm install documentdb-operator ./documentdb-chart \
     --namespace documentdb-operator \
     --create-namespace
   ```

## Cluster Management

### Kind Cluster Operations

```bash
# Create cluster with custom config
kind create cluster --name documentdb-dev --config kind-config.yaml

# List clusters
kind get clusters

# Delete cluster
kind delete cluster --name documentdb-dev

# Load Docker image into cluster
kind load docker-image controller:latest --name documentdb-dev

# Export kubeconfig
kind export kubeconfig --name documentdb-dev
```

### Useful kubectl Commands

```bash
# Watch pods in all namespaces
kubectl get pods -A -w

# Get operator logs
kubectl logs -f deployment/documentdb-operator-controller-manager -n documentdb-operator-system

# Describe DocumentDB resources
kubectl get documentdb -A
kubectl describe documentdb <name> -n <namespace>

# Port forward to DocumentDB service
kubectl port-forward pod/<documentdb-pod> 10260:10260 -n <namespace>
```

## Troubleshooting

### Common Issues

1. **"kind cluster not found"**
   ```bash
   # Create a new cluster
   kind create cluster --name documentdb-dev
   ```

2. **"docker daemon not running"**
   - Make sure Docker Desktop is running
   - In DevContainer: Docker-in-Docker should handle this automatically

3. **"kubectl: command not found"**
   ```bash
   # In DevContainer, this should be pre-installed
   # For manual setup, install kubectl
   ```

4. **"make: command not found"**
   ```bash
   # Install make (usually pre-installed on Linux/macOS)
   sudo apt-get install make  # Ubuntu/Debian
   ```

5. **Permission denied errors with Docker**
   ```bash
   # Add user to docker group (Linux)
   sudo usermod -aG docker $USER
   # Then logout and login again
   ```

### Getting Help

- **Verify your environment setup:**
  ```bash
  .devcontainer/verify-environment.sh
  ```

- **View available make targets:**
  ```bash
  make help
  ```

- **Check Go environment:**
  ```bash
  go env
  ```

- **Verify cluster connectivity:**
  ```bash
  kubectl cluster-info
  kubectl get nodes
  ```

- **Check operator status:**
  ```bash
  kubectl get pods -n documentdb-operator-system
  kubectl get crd | grep documentdb
  ```

### Development Tips

1. **Use watch mode for continuous testing:**
   ```bash
   # Watch file changes and run tests
   find . -name "*.go" | entr -c make test
   ```

2. **Quick iteration cycle:**
   ```bash
   make build && make docker-build && kind load docker-image controller:latest --name documentdb-dev && make deploy
   ```

3. **Debug operator locally:**
   ```bash
   # Run operator outside cluster for debugging
   make install  # Install CRDs
   make run      # Run operator locally
   ```

4. **Use separate namespaces for testing:**
   ```bash
   kubectl create namespace documentdb-dev-1
   kubectl create namespace documentdb-dev-2
   # Deploy test resources to different namespaces
   ```

## Contributing

When contributing to the project:

1. **Follow the established patterns:**
   - Use the existing Makefile targets
   - Follow Go coding standards
   - Add tests for new functionality

2. **Before submitting PR:**
   ```bash
   make fmt
   make vet
   make lint
   make test
   make test-e2e  # If applicable
   ```

3. **Update documentation:**
   - Update this guide if adding new workflows
   - Update README.md for user-facing changes
   - Add inline code comments for complex logic

For more information, see [CONTRIBUTING.md](../CONTRIBUTING.md).