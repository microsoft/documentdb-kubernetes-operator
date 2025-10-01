#!/bin/bash

# DocumentDB EKS Cluster Deletion Script
# This script completely removes the EKS cluster and all AWS resources to avoid charges

set -e  # Exit on any error

# Configuration
CLUSTER_NAME="documentdb-cluster"
REGION="us-west-2"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --region)
            REGION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --cluster-name NAME   EKS cluster name (default: documentdb-cluster)"
            echo "  --region REGION       AWS region (default: us-west-2)"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Delete default cluster"
            echo "  $0 --cluster-name my-cluster          # Delete custom cluster"
            echo "  $0 --region us-east-1                 # Delete in different region"
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
}

# Confirmation prompt
confirm_deletion() {
    echo ""
    echo "======================================="
    echo "    CLUSTER DELETION WARNING"
    echo "======================================="
    echo ""
    warn "This will DELETE the following resources:"
    echo "  • EKS Cluster: $CLUSTER_NAME"
    echo "  • All DocumentDB instances"
    echo "  • All operator deployments"
    echo "  • All persistent volumes"
    echo "  • Load balancers and networking"
    echo "  • IAM roles and policies"
    echo ""
    warn "This action is IRREVERSIBLE!"
    echo ""
    
    read -p "Are you sure you want to delete everything? (type 'yes' to confirm): " confirmation
    
    if [ "$confirmation" != "yes" ]; then
        log "Deletion cancelled by user"
        exit 0
    fi
    
    log "Proceeding with cluster deletion..."
}

# Delete DocumentDB instances
delete_documentdb_instances() {
    log "Deleting DocumentDB instances..."
    
    # Delete all DocumentDB instances
    kubectl delete documentdb --all --all-namespaces --timeout=300s || warn "No DocumentDB instances found or deletion failed"
    
    # Wait for PostgreSQL clusters to be deleted
    log "Waiting for PostgreSQL clusters to be deleted..."
    sleep 30
    
    success "DocumentDB instances deleted"
}

# Delete Helm releases
delete_helm_releases() {
    log "Deleting Helm releases..."
    
    # Delete DocumentDB operator (check both possible namespaces)
    helm uninstall documentdb-operator -n documentdb-system 2>/dev/null || warn "DocumentDB operator not found in documentdb-system namespace"
    helm uninstall documentdb-operator -n documentdb-operator 2>/dev/null || warn "DocumentDB operator not found in documentdb-operator namespace"
    
    # Delete AWS Load Balancer Controller
    helm uninstall aws-load-balancer-controller -n kube-system 2>/dev/null || warn "AWS Load Balancer Controller not found"
    
    # Delete cert-manager
    helm uninstall cert-manager -n cert-manager 2>/dev/null || warn "cert-manager not found"
    
    # Give some time for resources to be cleaned up
    log "Waiting for Helm releases to be fully removed..."
    sleep 15
    
    success "Helm releases deleted"
}

# Delete namespaces
delete_namespaces() {
    log "Deleting namespaces..."
    
    # Delete namespaces
    kubectl delete namespace documentdb-operator --timeout=300s || warn "documentdb-operator namespace not found"
    kubectl delete namespace documentdb-system --timeout=300s || warn "documentdb-system namespace not found"
    kubectl delete namespace cert-manager --timeout=300s || warn "cert-manager namespace not found"
    
    success "Namespaces deleted"
}

# Delete CRDs
delete_crds() {
    log "Deleting Custom Resource Definitions..."
    
    # Delete DocumentDB CRDs
    kubectl delete crd documentdbs.db.microsoft.com || warn "DocumentDB CRDs not found"
    
    # Delete PostgreSQL CRDs
    kubectl delete crd -l app.kubernetes.io/name=cloudnative-pg || warn "PostgreSQL CRDs not found"
    
    # Delete cert-manager CRDs
    kubectl delete crd -l app.kubernetes.io/name=cert-manager || warn "cert-manager CRDs not found"
    
    success "CRDs deleted"
}

# Delete AWS resources
delete_aws_resources() {
    log "Deleting AWS resources..."
    
    # Get account ID
    ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text 2>/dev/null) || {
        warn "Could not get AWS account ID. Skipping IAM policy deletion."
        return 0
    }
    
    # Delete IAM policies (only if they exist)
    aws iam delete-policy --policy-arn arn:aws:iam::$ACCOUNT_ID:policy/AWSLoadBalancerControllerIAMPolicy 2>/dev/null || warn "IAM policy AWSLoadBalancerControllerIAMPolicy not found"
    
    # Delete any remaining load balancers
    log "Checking for remaining load balancers..."
    local remaining_lbs=$(aws elbv2 describe-load-balancers --region $REGION --query "LoadBalancers[?contains(LoadBalancerName, 'k8s')].LoadBalancerArn" --output text 2>/dev/null || echo "")
    if [ -n "$remaining_lbs" ]; then
        warn "Found remaining load balancers. They may take a few minutes to delete automatically."
    fi
    
    # Delete any remaining volumes
    log "Checking for remaining EBS volumes..."
    local remaining_volumes=$(aws ec2 describe-volumes --region $REGION --filters "Name=tag:kubernetes.io/cluster/$CLUSTER_NAME,Values=owned" --query "Volumes[?State=='available'].VolumeId" --output text 2>/dev/null || echo "")
    if [ -n "$remaining_volumes" ]; then
        warn "Found remaining EBS volumes. Attempting to delete them..."
        for volume in $remaining_volumes; do
            aws ec2 delete-volume --volume-id $volume --region $REGION 2>/dev/null || warn "Could not delete volume $volume"
        done
    fi
    
    success "AWS resources cleanup attempted"
}

# Delete EKS cluster
delete_cluster() {
    log "Deleting EKS cluster..."
    
    # Check if cluster exists
    if ! eksctl get cluster --name $CLUSTER_NAME --region $REGION &> /dev/null; then
        warn "Cluster $CLUSTER_NAME not found. Skipping cluster deletion."
        return 0
    fi
    
    # Delete the cluster
    eksctl delete cluster --name $CLUSTER_NAME --region $REGION --wait
    
    if [ $? -eq 0 ]; then
        success "EKS cluster deleted successfully"
    else
        error "Failed to delete EKS cluster"
    fi
}

# Clean up local kubectl context
cleanup_kubectl_context() {
    log "Cleaning up kubectl context..."
    
    # Remove kubectl context (handle both possible context names)
    kubectl config delete-context "$CLUSTER_NAME.$REGION.eksctl.io" 2>/dev/null || warn "kubectl context $CLUSTER_NAME.$REGION.eksctl.io not found"
    kubectl config delete-cluster "$CLUSTER_NAME.$REGION.eksctl.io" 2>/dev/null || warn "kubectl cluster $CLUSTER_NAME.$REGION.eksctl.io not found"
    kubectl config delete-user "documentdb-admin@$CLUSTER_NAME.$REGION.eksctl.io" 2>/dev/null || warn "kubectl user not found"
    
    # Also try the default user pattern
    kubectl config delete-user "$CLUSTER_NAME@$CLUSTER_NAME.$REGION.eksctl.io" 2>/dev/null || warn "kubectl user (alternate pattern) not found"
    
    success "kubectl context cleaned up"
}

# Verify deletion
verify_deletion() {
    log "Verifying deletion..."
    
    echo ""
    echo "=== Checking for remaining resources ==="
    
    # Check if cluster exists
    if eksctl get cluster --name $CLUSTER_NAME --region $REGION &> /dev/null; then
        warn "Cluster still exists!"
    else
        success "Cluster deleted"
    fi
    
    # Check for remaining CloudFormation stacks
    echo ""
    log "Checking for remaining CloudFormation stacks..."
    aws cloudformation list-stacks --region $REGION --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE --query "StackSummaries[?contains(StackName, 'eksctl-$CLUSTER_NAME')].{Name:StackName,Status:StackStatus}" --output table || true
    
    # Check for remaining EBS volumes
    echo ""
    log "Checking for remaining EBS volumes..."
    aws ec2 describe-volumes --region $REGION --filters "Name=tag:kubernetes.io/cluster/$CLUSTER_NAME,Values=owned" --query "Volumes[].{VolumeId:VolumeId,State:State,Size:Size}" --output table 2>/dev/null || log "No volumes found with cluster tag"
    
    # Check for remaining load balancers
    echo ""
    log "Checking for remaining load balancers..."
    aws elbv2 describe-load-balancers --region $REGION --query "LoadBalancers[?contains(LoadBalancerName, 'k8s')].[LoadBalancerName,State.Code]" --output table 2>/dev/null || log "No load balancers found"
    
    echo ""
    success "Deletion verification complete!"
}

# Manual cleanup instructions
show_manual_cleanup() {
    echo ""
    echo "======================================="
    echo "    MANUAL CLEANUP (if needed)"
    echo "======================================="
    echo ""
    echo "If any resources remain, you can manually clean them up:"
    echo ""
    echo "1. CloudFormation Stacks:"
    echo "   aws cloudformation delete-stack --stack-name STACK_NAME --region $REGION"
    echo ""
    echo "2. EBS Volumes:"
    echo "   aws ec2 delete-volume --volume-id VOLUME_ID --region $REGION"
    echo ""
    echo "3. Load Balancers:"
    echo "   aws elbv2 delete-load-balancer --load-balancer-arn LOAD_BALANCER_ARN"
    echo ""
    echo "4. IAM Roles and Policies:"
    echo "   Check AWS Console -> IAM for any remaining eksctl-created resources"
    echo ""
}

# Main execution
main() {
    echo "======================================="
    echo "    DocumentDB EKS Cluster Deletion"
    echo "======================================="
    echo ""
    log "Target Configuration:"
    log "  Cluster: $CLUSTER_NAME"  
    log "  Region: $REGION"
    echo ""
    
    confirm_deletion
    
    log "Starting cluster deletion process..."
    
    # Check if cluster exists before proceeding
    if ! eksctl get cluster --name $CLUSTER_NAME --region $REGION &> /dev/null; then
        warn "Cluster '$CLUSTER_NAME' not found in region '$REGION'"
        log "This may have been already deleted, or the name/region is incorrect."
        log "Proceeding with cleanup of any remaining local resources..."
        cleanup_kubectl_context
        return 0
    fi
    
    delete_documentdb_instances
    delete_helm_releases
    delete_namespaces
    delete_crds
    delete_aws_resources
    delete_cluster
    cleanup_kubectl_context
    verify_deletion
    
    echo ""
    echo "======================================="
    success "🗑️  Cluster deletion completed!"
    echo "======================================="
    echo ""
    echo "Summary:"
    echo "  • EKS cluster '$CLUSTER_NAME' deleted from $REGION"
    echo "  • All DocumentDB instances removed"
    echo "  • All AWS resources cleaned up"
    echo "  • kubectl context removed"
    echo ""
    success "No more AWS charges should be incurred from this cluster!"
    echo ""
    
    show_manual_cleanup
}

# Run main function
main "$@"