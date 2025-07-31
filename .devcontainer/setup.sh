#!/bin/bash

# DocumentDB Kubernetes Operator Development Environment Setup Script
# This script sets up additional tools and configurations needed for development

# Enable debug output if DEBUG is set
if [[ "${DEBUG:-false}" == "true" ]]; then
    set -x
fi

# Function to log messages with timestamps
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to handle errors
handle_error() {
    log "❌ Error occurred in setup script at line $1"
    log "💡 You can set DEBUG=true to get more detailed output"
    log "💡 You can also run the setup script manually: .devcontainer/setup.sh"
    exit 1
}

# Set up error handling
set -e
trap 'handle_error $LINENO' ERR

log "🚀 Setting up DocumentDB Kubernetes Operator development environment..."

# Ensure we're in the workspace directory
log "📂 Navigating to workspace directory..."
if [[ -d "/workspaces/documentdb-kubernetes-operator" ]]; then
    cd /workspaces/documentdb-kubernetes-operator
    log "✅ Using workspace directory: /workspaces/documentdb-kubernetes-operator"
elif [[ -d "/workspace" ]]; then
    cd /workspace
    log "✅ Using workspace directory: /workspace"
else
    log "⚠️  Could not find standard workspace directories, staying in current directory: $(pwd)"
fi

# Verify Go is available
log "🔍 Checking Go installation..."
if ! command -v go &> /dev/null; then
    log "❌ Go is not installed or not in PATH"
    exit 1
fi
log "✅ Go version: $(go version)"

# Install additional Go tools that might be needed
log "📦 Installing additional Go tools..."
if ! go install -a github.com/golangci/golangci-lint/cmd/golangci-lint@latest; then
    log "⚠️  Failed to install golangci-lint, continuing..."
fi

if ! go install -a golang.org/x/tools/cmd/goimports@latest; then
    log "⚠️  Failed to install goimports, continuing..."
fi

# Pre-download Go modules to cache them
log "📥 Pre-downloading Go modules..."
if [[ -f "go.mod" ]]; then
    if ! go mod download; then
        log "⚠️  Failed to download Go modules, continuing..."
    fi
else
    log "⚠️  No go.mod file found, skipping module download"
fi

# Create kubeconfig directory
log "🔧 Setting up kubectl configuration..."
mkdir -p ~/.kube

# Set up Git safe directory (for when using bind mounts)
log "🔒 Setting up Git safe directory..."
# Get the actual workspace path
WORKSPACE_PATH=$(pwd)
if ! git config --global --add safe.directory "$WORKSPACE_PATH"; then
    log "⚠️  Failed to set Git safe directory, continuing..."
fi

# Install development dependencies through Make targets
log "🛠️  Installing development tools via Makefile..."
if [[ -f "Makefile" ]]; then
    make setup-envtest || log "⚠️  setup-envtest failed, continuing..."
    make controller-gen || log "⚠️  controller-gen failed, continuing..."
    make kustomize || log "⚠️  kustomize failed, continuing..."
else
    log "⚠️  No Makefile found, skipping make targets"
fi

# Print some helpful information
log ""
log "✅ Development environment setup complete!"
log ""
log "🔍 To verify your environment:"
log "   .devcontainer/verify-environment.sh    - Check all tools and configuration"
log ""
log "🔧 Available development commands:"
log "   make help           - Show all available make targets"
log "   make build          - Build the operator binary"
log "   make test           - Run unit tests"
log "   make test-e2e       - Run e2e tests (requires kind cluster)"
log "   make lint           - Run code linter"
log "   make deploy         - Deploy operator to current kubectl context"
log ""
log "🌐 Kind cluster management:"
log "   kind create cluster                    - Create a new kind cluster"
log "   kind get clusters                      - List existing kind clusters"
log "   kind delete cluster                    - Delete the kind cluster"
log "   kubectl cluster-info --context kind-kind - Verify cluster connection"
log ""
log "⚡ Quick start:"
log "   .devcontainer/quick-setup.sh           - Complete setup with sample DocumentDB"
log ""
log "📚 See docs/developer-guide.md for detailed development workflow"
log ""