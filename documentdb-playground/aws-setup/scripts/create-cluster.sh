#!/bin/bash

# DocumentDB EKS Cluster Creation Script
# This script creates a complete EKS cluster with all dependencies for DocumentDB

set -e  # Exit on any error

# Configuration
CLUSTER_NAME="documentdb-cluster"
REGION="us-west-2"
NODE_TYPE="m5.large"
NODES=2
NODES_MIN=1
NODES_MAX=4

# Feature flags - set to "true" to enable, "false" to skip
INSTALL_OPERATOR="${INSTALL_OPERATOR:-false}"
DEPLOY_INSTANCE="${DEPLOY_INSTANCE:-false}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-operator)
            INSTALL_OPERATOR="false"
            shift
            ;;
        --skip-instance)
            DEPLOY_INSTANCE="false"
            shift
            ;;
        --install-operator)
            INSTALL_OPERATOR="true"
            shift
            ;;
        --deploy-instance)
            DEPLOY_INSTANCE="true"
            INSTALL_OPERATOR="true"  # Auto-enable operator when instance is requested
            shift
            ;;
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --region)
            REGION="$2"
            shift 2
            ;;
        --github-username)
            GITHUB_USERNAME="$2"
            shift 2
            ;;
        --github-token)
            GITHUB_TOKEN="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-operator       Skip DocumentDB operator installation (default)"
            echo "  --skip-instance       Skip DocumentDB instance deployment (default)"
            echo "  --install-operator    Install DocumentDB operator"
            echo "  --deploy-instance     Deploy DocumentDB instance"
            echo "  --cluster-name NAME   EKS cluster name (default: documentdb-cluster)"
            echo "  --region REGION       AWS region (default: us-west-2)"
            echo "  --github-username     GitHub username for operator installation"
            echo "  --github-token        GitHub token for operator installation"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Create basic cluster only (no operator, no instance)"
            echo "  $0 --install-operator                 # Create cluster with operator, no instance"
            echo "  $0 --deploy-instance                  # Create cluster with instance (auto-enables operator)"
            echo "  $0 --github-username user --github-token ghp_xxx --install-operator  # With GitHub auth"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] ✅ $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] ⚠️  $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ❌ $1${NC}"
    exit 1
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        error "AWS CLI not found. Please install AWS CLI first."
    fi
    
    # Check eksctl
    if ! command -v eksctl &> /dev/null; then
        error "eksctl not found. Please install eksctl first."
    fi
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl not found. Please install kubectl first."
    fi
    
    # Check Helm
    if ! command -v helm &> /dev/null; then
        error "Helm not found. Please install Helm first."
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        error "AWS credentials not configured. Please run 'aws configure' first."
    fi
    
    success "All prerequisites met"
}

# Create EKS cluster
create_cluster() {
    log "Creating EKS cluster: $CLUSTER_NAME in region: $REGION"
    
    # Check if cluster already exists
    if eksctl get cluster --name $CLUSTER_NAME --region $REGION &> /dev/null; then
        warn "Cluster $CLUSTER_NAME already exists. Skipping cluster creation."
        return 0
    fi
    
    # Create cluster with basic configuration
    eksctl create cluster \
        --name $CLUSTER_NAME \
        --region $REGION \
        --node-type $NODE_TYPE \
        --nodes $NODES \
        --nodes-min $NODES_MIN \
        --nodes-max $NODES_MAX \
        --managed \
        --with-oidc
    
    if [ $? -eq 0 ]; then
        success "EKS cluster created successfully"
    else
        error "Failed to create EKS cluster"
    fi
}

# Install EBS CSI Driver
install_ebs_csi() {
    log "Installing EBS CSI Driver..."
    
    # Create EBS CSI service account with IAM role
    eksctl create iamserviceaccount \
        --cluster $CLUSTER_NAME \
        --namespace kube-system \
        --name ebs-csi-controller-sa \
        --attach-policy-arn arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy \
        --override-existing-serviceaccounts \
        --approve \
        --region $REGION
    
    # Install EBS CSI driver addon
    eksctl create addon \
        --name aws-ebs-csi-driver \
        --cluster $CLUSTER_NAME \
        --region $REGION \
        --force
    
    # Wait for EBS CSI driver to be ready
    log "Waiting for EBS CSI driver to be ready..."
    sleep 30
    kubectl wait --for=condition=ready pod -l app=ebs-csi-controller -n kube-system --timeout=300s || warn "EBS CSI driver pods may still be starting"
    
    success "EBS CSI Driver installed"
}

# Install AWS Load Balancer Controller
install_load_balancer_controller() {
    log "Installing AWS Load Balancer Controller..."
    
    # Check if already installed
    if helm list -n kube-system | grep -q aws-load-balancer-controller; then
        warn "AWS Load Balancer Controller already installed. Skipping installation."
        return 0
    fi
    
    # Download IAM policy
    curl -o /tmp/iam_policy.json https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.7.2/docs/install/iam_policy.json
    
    # Create IAM policy
    aws iam create-policy \
        --policy-name AWSLoadBalancerControllerIAMPolicy \
        --policy-document file:///tmp/iam_policy.json 2>/dev/null || true
    
    # Create service account
    eksctl create iamserviceaccount \
        --cluster $CLUSTER_NAME \
        --namespace kube-system \
        --name aws-load-balancer-controller \
        --attach-policy-arn arn:aws:iam::$(aws sts get-caller-identity --query Account --output text):policy/AWSLoadBalancerControllerIAMPolicy \
        --approve \
        --region $REGION
    
    # Add EKS Helm repository
    helm repo add eks https://aws.github.io/eks-charts
    helm repo update
    
    # Install Load Balancer Controller
    helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
        -n kube-system \
        --set clusterName=$CLUSTER_NAME \
        --set serviceAccount.create=false \
        --set serviceAccount.name=aws-load-balancer-controller \
        --set region=$REGION
    
    # Wait for Load Balancer Controller to be ready
    log "Waiting for Load Balancer Controller to be ready..."
    sleep 30
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=aws-load-balancer-controller -n kube-system --timeout=300s || warn "Load Balancer Controller pods may still be starting"
    
    # Clean up temp file
    rm -f /tmp/iam_policy.json
    
    success "AWS Load Balancer Controller installed"
}

# Install cert-manager
install_cert_manager() {
    log "Installing cert-manager..."
    
    # Check if already installed
    if helm list -n cert-manager | grep -q cert-manager; then
        warn "cert-manager already installed. Skipping installation."
        return 0
    fi
    
    # Add Jetstack Helm repository
    helm repo add jetstack https://charts.jetstack.io
    helm repo update
    
    # Install cert-manager
    helm install cert-manager jetstack/cert-manager \
        --namespace cert-manager \
        --create-namespace \
        --version v1.13.2 \
        --set installCRDs=true \
        --set prometheus.enabled=false \
        --set webhook.timeoutSeconds=30
    
    # Wait for cert-manager to be ready
    log "Waiting for cert-manager to be ready..."
    sleep 30
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s || warn "cert-manager pods may still be starting"
    
    success "cert-manager installed"
}

# Create optimized storage class
create_storage_class() {
    log "Creating DocumentDB storage class..."
    
    # Check if storage class already exists
    if kubectl get storageclass documentdb-storage &> /dev/null; then
        warn "DocumentDB storage class already exists. Skipping creation."
        return 0
    fi
    
    kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: documentdb-storage
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
  fsType: ext4
  encrypted: "true"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Retain
EOF
    
    success "DocumentDB storage class created"
}

# Install DocumentDB operator (optional)
install_documentdb_operator() {
    if [ "$INSTALL_OPERATOR" != "true" ]; then
        warn "Skipping DocumentDB operator installation (--skip-operator specified)"
        return 0
    fi
    
    log "Installing DocumentDB operator from official GitHub registry..."
    
    # Check if operator is already installed
    if helm list -n documentdb-operator | grep -q documentdb-operator; then
        warn "DocumentDB operator already installed. Skipping installation."
        return 0
    fi
    
    # Test internet connectivity to GitHub registry
    log "Testing connectivity to GitHub Container Registry..."
    if ! curl -s --connect-timeout 10 https://ghcr.io > /dev/null; then
        error "Cannot reach ghcr.io. Please check your internet connection and firewall settings."
    fi
    
    # Install DocumentDB operator using official OCI registry
    log "Installing DocumentDB operator from GitHub Container Registry..."
    
    # Check for GitHub authentication
    if [ -z "$GITHUB_TOKEN" ] || [ -z "$GITHUB_USERNAME" ]; then
        error "DocumentDB operator installation requires GitHub authentication.

Please set the following environment variables:
  export GITHUB_USERNAME='your-github-username'
  export GITHUB_TOKEN='your-github-token'

To create a GitHub token:
1. Go to https://github.com/settings/tokens
2. Generate a new token with 'read:packages' scope
3. Export the token as shown above

Then run the script again with --install-operator"
    fi
    
    # Authenticate with GitHub Container Registry
    log "Authenticating with GitHub Container Registry..."
    if ! echo "$GITHUB_TOKEN" | helm registry login ghcr.io --username "$GITHUB_USERNAME" --password-stdin; then
        error "Failed to authenticate with GitHub Container Registry. Please verify your GITHUB_TOKEN and GITHUB_USERNAME."
    fi
    
    # Install DocumentDB operator from OCI registry
    log "Pulling and installing DocumentDB operator from ghcr.io/microsoft/documentdb-operator..."
    helm install documentdb-operator \
        oci://ghcr.io/microsoft/documentdb-operator \
        --version 0.0.1 \
        --namespace documentdb-operator \
        --create-namespace \
        --wait \
        --timeout 10m

    if [ $? -eq 0 ]; then
        success "DocumentDB operator installed successfully from official registry"
    else
        error "Failed to install DocumentDB operator from OCI registry. Please verify:
- Your GitHub token has 'read:packages' scope
- You have access to microsoft/documentdb-operator repository  
- The chart version 0.0.1 exists"
    fi    # Wait for operator to be ready
    log "Waiting for DocumentDB operator to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=documentdb-operator -n documentdb-operator --timeout=300s || warn "DocumentDB operator pods may still be starting"
    
    success "DocumentDB operator installed"
}

# Deploy DocumentDB instance (optional)
deploy_documentdb_instance() {
    if [ "$DEPLOY_INSTANCE" != "true" ]; then
        warn "Skipping DocumentDB instance deployment (--skip-instance specified or not enabled)"
        return 0
    fi
    
    log "Deploying DocumentDB instance..."
    
    # Check if operator is installed
    if ! kubectl get deployment -n documentdb-operator documentdb-operator &> /dev/null; then
        error "DocumentDB operator not found. Cannot deploy instance without operator."
    fi
    
    # Create credentials secret
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: documentdb-credentials
  namespace: default
type: Opaque
data:
  username: $(echo -n "docdbadmin" | base64)
  password: $(echo -n "SecurePassword123!" | base64)
EOF
    
    # Deploy DocumentDB instance
    kubectl apply -f - <<EOF
apiVersion: db.microsoft.com/v1alpha1
kind: DocumentDB
metadata:
  name: sample-documentdb
  namespace: default
spec:
  replicas: 1
  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "1000m"
  storage:
    size: "10Gi"
    storageClass: "documentdb-storage"
  auth:
    secretRef:
      name: "documentdb-credentials"
EOF
    
    # Wait for DocumentDB to be ready
    log "Waiting for DocumentDB instance to be ready (this may take several minutes)..."
    kubectl wait --for=condition=ready documentdb sample-documentdb --timeout=600s || warn "DocumentDB instance may still be starting"
    
    success "DocumentDB instance deployed"
    
    # Show connection info
    log "DocumentDB instance connection information:"
    kubectl get documentdb sample-documentdb -o wide
}

# Print summary
print_summary() {
    echo ""
    echo "=================================================="
    echo "🎉 CLUSTER SETUP COMPLETE!"
    echo "=================================================="
    echo "Cluster Name: $CLUSTER_NAME"
    echo "Region: $REGION"
    echo "Operator Installed: $INSTALL_OPERATOR"
    echo "Instance Deployed: $DEPLOY_INSTANCE"
    echo ""
    echo "✅ Components installed:"
    echo "  - EKS cluster with managed nodes"
    echo "  - EBS CSI driver"
    echo "  - AWS Load Balancer Controller"
    echo "  - cert-manager"
    echo "  - DocumentDB storage class"
    if [ "$INSTALL_OPERATOR" == "true" ]; then
        echo "  - DocumentDB operator"
    fi
    if [ "$DEPLOY_INSTANCE" == "true" ]; then
        echo "  - DocumentDB instance (sample-documentdb)"
    fi
    echo ""
    echo "💡 Next steps:"
    echo "  - Verify cluster: kubectl get nodes"
    echo "  - Check all pods: kubectl get pods --all-namespaces"
    if [ "$INSTALL_OPERATOR" == "true" ]; then
        echo "  - Check operator: kubectl get pods -n documentdb-operator"
    fi
    if [ "$DEPLOY_INSTANCE" == "true" ]; then
        echo "  - Check DocumentDB: kubectl get documentdb"
        echo "  - Test connection: kubectl port-forward svc/sample-documentdb 27017:27017"
    fi
    echo ""
    echo "⚠️  IMPORTANT: Run './delete-cluster.sh' when done to avoid AWS charges!"
    echo "=================================================="
}

# Main execution
main() {
    log "Starting DocumentDB EKS cluster setup..."
    log "Configuration:"
    log "  Cluster: $CLUSTER_NAME"
    log "  Region: $REGION"
    log "  Install Operator: $INSTALL_OPERATOR"
    log "  Deploy Instance: $DEPLOY_INSTANCE"
    echo ""
    
    # Execute setup steps
    check_prerequisites
    create_cluster
    install_ebs_csi
    install_load_balancer_controller
    install_cert_manager
    create_storage_class
    
    # Optional components
    install_documentdb_operator
    deploy_documentdb_instance
    
    # Show summary
    print_summary
}

# Run main function
main "$@"
