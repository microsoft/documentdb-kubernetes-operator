# DocumentDB Kubernetes Operator - AWS EKS Deployment

Simple automation scripts for deploying DocumentDB operator on AWS EKS.

## Prerequisites

- AWS CLI configured: `aws configure`
- Required tools: `aws`, `eksctl`, `kubectl`, `helm`, `jq`
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
# Create EKS cluster with DocumentDB (includes public IP LoadBalancer)
export GITHUB_USERNAME="your-github-username"
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
./scripts/create-cluster.sh --deploy-instance

# Delete cluster when done (avoid charges)
./scripts/delete-cluster.sh

# OR keep cluster and delete DocumentDB components
./scripts/delete-cluster.sh --instance-and-operator
```

## Load Balancer Configuration

The DocumentDB service is automatically configured with these annotations for public IP access:

```yaml
serviceAnnotations:
  service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
  service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
  service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
  service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: "ip"
```

**Note**: It takes 2-5 minutes for AWS to provision the Network Load Balancer and assign a public IP.

### Manual Service Patching

If you need to manually add LoadBalancer annotations to an existing DocumentDB service:

```bash
# Auto-detect and patch DocumentDB service
./scripts/patch-documentdb-service.sh

# Patch specific service
./scripts/patch-documentdb-service.sh --service-name my-documentdb-svc --namespace my-namespace
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
# Delete everything (default)
./scripts/delete-cluster.sh

# Delete only DocumentDB instances (keep operator and cluster)
./scripts/delete-cluster.sh --instance-only

# Delete instances and operator (keep cluster)
./scripts/delete-cluster.sh --instance-and-operator

# Custom configuration  
./scripts/delete-cluster.sh --cluster-name my-cluster --region us-east-1

# See all options
./scripts/delete-cluster.sh --help
```

**Available options:**
- `--cluster-name NAME` - EKS cluster name (default: documentdb-cluster)
- `--region REGION` - AWS region (default: us-west-2)
- `--instance-only` - Delete only DocumentDB instances
- `--instance-and-operator` - Delete instances and operator (keep cluster)

**Common scenarios:**
- **Default**: Delete everything (instances + operator + cluster)
- **Cost optimization**: Use `--instance-and-operator` to preserve expensive EKS setup
- **Testing instances**: Use `--instance-only` to test deployments without recreating operator
- **Operator upgrades**: Use `--instance-and-operator` to reinstall operator without losing cluster

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