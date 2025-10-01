# DocumentDB Kubernetes Operator - AWS EKS Deployment

Simple automation scripts for deploying DocumentDB operator on AWS EKS.

## Prerequisites

- AWS CLI configured: `aws configure`
- Required tools: `aws`, `eksctl`, `kubectl`, `helm`
- **For operator installation**: GitHub account with access to microsoft/documentdb-operator

### GitHub Authentication (Required for Operator)

To install the DocumentDB operator, you need GitHub Container Registry access:

1. **Create GitHub Personal Access Token**:
   - Go to https://github.com/settings/tokens
   - Click "Generate new token (classic)"
   - Select scope: `read:packages`
   - Copy the generated token

2. **Set Environment Variables**:
   ```bash
   export GITHUB_USERNAME="your-github-username"
   export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
   ```

## Quick Start

```bash
# Create EKS cluster (basic setup - ~$140/month)
./scripts/create-cluster.sh

# Delete cluster when done (avoid charges)
./scripts/delete-cluster.sh
```

## Script Options

### create-cluster.sh
```bash
# Basic usage (cluster only)
./scripts/create-cluster.sh

# With operator using environment variables
export GITHUB_USERNAME="your-username"
export GITHUB_TOKEN="your-token"
./scripts/create-cluster.sh --install-operator

# With operator using command-line parameters
./scripts/create-cluster.sh --install-operator \
  --github-username "your-username" \
  --github-token "your-token"

# Custom configuration
./scripts/create-cluster.sh --cluster-name my-cluster --region us-east-1

# See all options
./scripts/create-cluster.sh --help
```

**Available options:**
- `--cluster-name NAME` - EKS cluster name (default: documentdb-cluster)
- `--region REGION` - AWS region (default: us-west-2)
- `--skip-operator` - Skip operator installation (default)
- `--install-operator` - Install operator (requires GitHub authentication)
- `--deploy-instance` - Deploy operator + instance (requires GitHub authentication)

### delete-cluster.sh
```bash
# Basic usage
./scripts/delete-cluster.sh

# Custom configuration  
./scripts/delete-cluster.sh --cluster-name my-cluster --region us-east-1

# See all options
./scripts/delete-cluster.sh --help
```

**Available options:**
- `--cluster-name NAME` - EKS cluster name (default: documentdb-cluster)
- `--region REGION` - AWS region (default: us-west-2)

## What Gets Created

**create-cluster.sh builds:**
- EKS cluster with 2 managed nodes (m5.large)
- EBS CSI driver for storage
- AWS Load Balancer Controller
- cert-manager for TLS
- Optimized storage classes

**Estimated cost:** ~$140-230/month (always run delete-cluster.sh when done!)

## Support

- [GitHub Issues](https://github.com/microsoft/documentdb-kubernetes-operator/issues)
- [Documentation](https://microsoft.github.io/documentdb-kubernetes-operator)