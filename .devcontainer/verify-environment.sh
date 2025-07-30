#!/bin/bash
# DevContainer Environment Verification Script
# This script verifies that all required tools are properly installed and configured

set -e

echo "🔍 Verifying DocumentDB Kubernetes Operator development environment..."
echo ""

errors=0

# Function to check if command exists
check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        version=$($1 version 2>/dev/null | head -1 || echo "installed")
        echo "✅ $1: $version"
    else
        echo "❌ $1: not found"
        ((errors++))
    fi
}

# Function to check if command exists with custom version command
check_command_custom() {
    if command -v "$1" >/dev/null 2>&1; then
        version=$(eval "$2" 2>/dev/null | head -1 || echo "installed")
        echo "✅ $1: $version"
    else
        echo "❌ $1: not found"
        ((errors++))
    fi
}

# Check Go
if command -v go >/dev/null 2>&1; then
    go_version=$(go version)
    if [[ $go_version == *"go1.23"* ]] || [[ $go_version == *"go1.24"* ]] || [[ $go_version > *"go1.23"* ]]; then
        echo "✅ Go: $go_version"
    else
        echo "⚠️  Go: $go_version (expected 1.23+)"
        ((errors++))
    fi
else
    echo "❌ Go: not found"
    ((errors++))
fi

# Check other tools
check_command docker
check_command kubectl
check_command kind
check_command helm
check_command make

# Check Go tools (these might not be installed initially)
echo ""
echo "🔧 Checking Go development tools (installed by setup script):"
if command -v golangci-lint >/dev/null 2>&1; then
    check_command_custom golangci-lint "golangci-lint version --format short"
else
    echo "ℹ️  golangci-lint: not installed (will be installed by .devcontainer/setup.sh)"
fi

if command -v goimports >/dev/null 2>&1; then
    check_command goimports
else
    echo "ℹ️  goimports: not installed (will be installed by .devcontainer/setup.sh)"
fi

# Check if we're in the right directory
if [ -f "go.mod" ] && grep -q "github.com/microsoft/documentdb-operator" go.mod; then
    echo "✅ Working directory: DocumentDB Kubernetes Operator repository"
else
    echo "⚠️  Working directory: not in DocumentDB Kubernetes Operator repository"
fi

# Check if Makefile exists and has expected targets
if [ -f "Makefile" ]; then
    if grep -q "build:" Makefile && grep -q "test:" Makefile && grep -q "deploy:" Makefile; then
        echo "✅ Makefile: contains expected targets"
    else
        echo "⚠️  Makefile: missing expected targets"
    fi
else
    echo "❌ Makefile: not found"
    ((errors++))
fi

# Test basic make targets
echo ""
echo "🧪 Testing basic development workflow..."

if make --dry-run build >/dev/null 2>&1; then
    echo "✅ make build: syntax OK"
else
    echo "❌ make build: syntax error"
    ((errors++))
fi

if make --dry-run test >/dev/null 2>&1; then
    echo "✅ make test: syntax OK"
else
    echo "❌ make test: syntax error"
    ((errors++))
fi

# Check kubectl context (if any cluster is configured)
if kubectl config current-context >/dev/null 2>&1; then
    context=$(kubectl config current-context)
    echo "✅ kubectl context: $context"
    
    if kubectl cluster-info >/dev/null 2>&1; then
        echo "✅ Kubernetes cluster: accessible"
    else
        echo "⚠️  Kubernetes cluster: configured but not accessible"
    fi
else
    echo "ℹ️  kubectl context: not configured (OK for initial setup)"
fi

echo ""
if [ $errors -eq 0 ]; then
    echo "🎉 Environment verification passed! You're ready for development."
    echo ""
    echo "📋 Next steps:"
    echo "   make build          # Build the operator"
    echo "   make test           # Run tests"
    echo "   .devcontainer/quick-setup.sh  # Complete setup with kind cluster"
else
    echo "❌ Environment verification failed with $errors error(s)."
    echo "   Please check the failed items above and refer to docs/developer-guide.md"
    exit 1
fi