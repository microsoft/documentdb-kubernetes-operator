#!/bin/bash

# DocumentDB Kubernetes Operator Development Environment Setup Script
# This script sets up additional tools and configurations needed for development

set -e

echo "🚀 Setting up DocumentDB Kubernetes Operator development environment..."

# Ensure we're in the workspace directory
cd /workspaces/documentdb-kubernetes-operator 2>/dev/null || cd /workspace 2>/dev/null || true

# Install additional Go tools that might be needed
echo "📦 Installing additional Go tools..."
go install -a github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install -a golang.org/x/tools/cmd/goimports@latest

# Pre-download Go modules to cache them
echo "📥 Pre-downloading Go modules..."
go mod download

# Create kubeconfig directory
echo "🔧 Setting up kubectl configuration..."
mkdir -p ~/.kube

# Set up Git safe directory (for when using bind mounts)
echo "🔒 Setting up Git safe directory..."
git config --global --add safe.directory /workspaces/documentdb-kubernetes-operator

# Install development dependencies through Make targets
echo "🛠️  Installing development tools via Makefile..."
make setup-envtest || true
make controller-gen || true
make kustomize || true

# Print some helpful information
echo ""
echo "✅ Development environment setup complete!"
echo ""
echo "🔍 To verify your environment:"
echo "   .devcontainer/verify-environment.sh    - Check all tools and configuration"
echo ""
echo "🔧 Available development commands:"
echo "   make help           - Show all available make targets"
echo "   make build          - Build the operator binary"
echo "   make test           - Run unit tests"
echo "   make test-e2e       - Run e2e tests (requires kind cluster)"
echo "   make lint           - Run code linter"
echo "   make deploy         - Deploy operator to current kubectl context"
echo ""
echo "🌐 Kind cluster management:"
echo "   kind create cluster                    - Create a new kind cluster"
echo "   kind get clusters                      - List existing kind clusters"
echo "   kind delete cluster                    - Delete the kind cluster"
echo "   kubectl cluster-info --context kind-kind - Verify cluster connection"
echo ""
echo "⚡ Quick start:"
echo "   .devcontainer/quick-setup.sh           - Complete setup with sample DocumentDB"
echo ""
echo "📚 See docs/developer-guide.md for detailed development workflow"
echo ""