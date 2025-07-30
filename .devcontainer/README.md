# DevContainer Setup for DocumentDB Kubernetes Operator

This directory contains the development container configuration for the DocumentDB Kubernetes Operator project.

## Quick Start

1. **Prerequisites:** 
   - [VS Code](https://code.visualstudio.com/) with [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
   - [Docker Desktop](https://www.docker.com/products/docker-desktop) or [Docker Engine](https://docs.docker.com/engine/install/)

2. **Open the project:** Clone the repository and open it in VS Code
3. **Start DevContainer:** Click "Reopen in Container" when prompted, or use Command Palette > "Dev Containers: Reopen in Container"
4. **Verify setup:** Run `.devcontainer/verify-environment.sh`
5. **Quick start:** Run `.devcontainer/quick-setup.sh` for complete setup with sample deployment

## What's Included

### Base Environment
- **Go 1.23** with development tools
- **Docker-in-Docker** for building container images
- **kubectl** for Kubernetes cluster management
- **Helm** for Kubernetes application management
- **kind** for local Kubernetes clusters

### VS Code Extensions
- Go language support with debugging
- Kubernetes and YAML tools
- Makefile support
- GitHub Copilot (if available)

### Development Tools
- golangci-lint (installed by setup script)
- goimports (installed by setup script)
- controller-gen, kustomize, envtest (installed via Makefile)

## Files Overview

- **`devcontainer.json`** - Main devcontainer configuration
- **`setup.sh`** - Post-create setup script that installs additional tools
- **`verify-environment.sh`** - Verification script to check all tools are properly installed
- **`quick-setup.sh`** - Complete setup script that creates cluster and deploys operator with sample
- **`kind-config.yaml`** - Optimized kind cluster configuration for development

## Usage

### Basic Development
```bash
# Build the operator
make build

# Run tests
make test

# Format and lint code
make fmt
make vet
make lint
```

### Full Environment Setup
```bash
# Create kind cluster, deploy operator, and create sample DocumentDB
.devcontainer/quick-setup.sh

# Or step by step:
kind create cluster --config .devcontainer/kind-config.yaml
make docker-build
kind load docker-image controller:latest
make deploy
```

### Testing
```bash
# Unit tests
make test

# E2E tests (requires kind cluster)
make test-e2e
```

## Troubleshooting

- **Container fails to start**: Check Docker is running and you have sufficient resources
- **Missing tools**: Run `.devcontainer/verify-environment.sh` to check what's missing
- **Build failures**: Ensure Go modules are properly downloaded with `go mod download`
- **Kind cluster issues**: Delete and recreate with `kind delete cluster && kind create cluster`

For more detailed troubleshooting, see the main [Developer Guide](../docs/developer-guide.md#troubleshooting).

## Customization

You can customize the devcontainer by modifying:
- `devcontainer.json` - Add VS Code extensions, change settings, add features
- `setup.sh` - Add additional tools or configuration
- `kind-config.yaml` - Modify Kubernetes cluster configuration

## Support

If you encounter issues with the devcontainer setup, please:
1. Run `.devcontainer/verify-environment.sh` and share the output
2. Check the [Developer Guide](../docs/developer-guide.md) for detailed troubleshooting
3. Open an issue in the repository with your environment details