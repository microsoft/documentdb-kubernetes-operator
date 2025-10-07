#!/bin/bash

# DocumentDB Service LoadBalancer Patch Script
# This script patches the DocumentDB service to add AWS LoadBalancer annotations
# for public IP access

set -e  # Exit on any error

# Configuration
CLUSTER_NAME="documentdb-cluster"
REGION="us-west-2"
NAMESPACE="documentdb-instance-ns"
SERVICE_NAME=""  # Will be auto-detected

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

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
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --service-name)
            SERVICE_NAME="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --cluster-name NAME   EKS cluster name (default: documentdb-cluster)"
            echo "  --region REGION       AWS region (default: us-west-2)"
            echo "  --namespace NS        DocumentDB namespace (default: documentdb-instance-ns)"
            echo "  --service-name NAME   Service name (auto-detected if not specified)"
            echo "  -h, --help           Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Patch service with default settings"
            echo "  $0 --namespace my-namespace           # Patch service in specific namespace"
            echo "  $0 --service-name my-documentdb-svc   # Patch specific service"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Find DocumentDB service if not specified
find_documentdb_service() {
    log "Looking for DocumentDB services in namespace $NAMESPACE..."
    
    # Look for services that might be DocumentDB services
    SERVICES=$(kubectl get services -n $NAMESPACE -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$SERVICES" ]; then
        error "No services found in namespace $NAMESPACE. Make sure DocumentDB instance is deployed first."
    fi
    
    # Look for services that contain 'documentdb' or are of type LoadBalancer
    for svc in $SERVICES; do
        # Check if it's a LoadBalancer type service
        SVC_TYPE=$(kubectl get service $svc -n $NAMESPACE -o jsonpath='{.spec.type}' 2>/dev/null || echo "")
        if [ "$SVC_TYPE" = "LoadBalancer" ]; then
            SERVICE_NAME="$svc"
            log "Found LoadBalancer service: $SERVICE_NAME"
            return 0
        fi
    done
    
    # If no LoadBalancer found, look for services with 'documentdb' in name
    for svc in $SERVICES; do
        if [[ $svc == *documentdb* ]]; then
            SERVICE_NAME="$svc"
            log "Found DocumentDB service: $SERVICE_NAME"
            return 0
        fi
    done
    
    # If still not found, just use the first service
    SERVICE_NAME=$(echo $SERVICES | awk '{print $1}')
    warn "No obvious DocumentDB service found, using first service: $SERVICE_NAME"
}

# Check if service has LoadBalancer annotations
check_service_annotations() {
    log "Checking current service annotations..."
    
    LB_TYPE=$(kubectl get service $SERVICE_NAME -n $NAMESPACE -o jsonpath='{.metadata.annotations.service\.beta\.kubernetes\.io/aws-load-balancer-type}' 2>/dev/null || echo "")
    LB_SCHEME=$(kubectl get service $SERVICE_NAME -n $NAMESPACE -o jsonpath='{.metadata.annotations.service\.beta\.kubernetes\.io/aws-load-balancer-scheme}' 2>/dev/null || echo "")
    
    if [ -n "$LB_TYPE" ] && [ "$LB_TYPE" = "nlb" ]; then
        success "Service already has LoadBalancer annotations"
        return 0
    else
        log "Service needs LoadBalancer annotations"
        return 1
    fi
}

# Patch service with LoadBalancer annotations
patch_service() {
    log "Patching service $SERVICE_NAME with LoadBalancer annotations..."
    
    # Create patch JSON
    PATCH_JSON='{
        "metadata": {
            "annotations": {
                "service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
                "service.beta.kubernetes.io/aws-load-balancer-scheme": "internet-facing",
                "service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled": "true",
                "service.beta.kubernetes.io/aws-load-balancer-nlb-target-type": "ip"
            }
        }
    }'
    
    # Apply patch
    kubectl patch service $SERVICE_NAME -n $NAMESPACE --type='merge' -p="$PATCH_JSON"
    
    if [ $? -eq 0 ]; then
        success "Service patched successfully"
    else
        error "Failed to patch service"
    fi
}

# Wait for LoadBalancer external IP
wait_for_external_ip() {
    log "Waiting for LoadBalancer external IP to be assigned..."
    
    for i in {1..20}; do  # Wait up to 10 minutes
        EXTERNAL_IP=$(kubectl get service $SERVICE_NAME -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
        
        if [ -n "$EXTERNAL_IP" ] && [ "$EXTERNAL_IP" != "<none>" ]; then
            success "External IP assigned: $EXTERNAL_IP"
            return 0
        fi
        
        log "Still waiting for external IP... (attempt $i/20)"
        sleep 30
    done
    
    warn "External IP not assigned within 10 minutes. This is normal for NLB provisioning."
    log "You can check the status later with: kubectl get service $SERVICE_NAME -n $NAMESPACE"
}

# Check pod status after patch
check_pod_status() {
    log "Checking DocumentDB pod status..."
    
    PODS=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=documentdb -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$PODS" ]; then
        log "No DocumentDB pods found yet. They should be created soon."
        return 0
    fi
    
    for pod in $PODS; do
        POD_STATUS=$(kubectl get pod $pod -n $NAMESPACE -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        log "Pod $pod status: $POD_STATUS"
        
        if [ "$POD_STATUS" = "Running" ]; then
            success "Pod $pod is running"
        elif [ "$POD_STATUS" = "Pending" ]; then
            log "Pod $pod is pending - this is expected during LoadBalancer provisioning"
        else
            warn "Pod $pod status: $POD_STATUS"
        fi
    done
}

# Main execution
main() {
    echo "======================================="
    echo "    DocumentDB Service Patch Tool"
    echo "======================================="
    echo ""
    
    log "Configuration:"
    log "  Cluster: $CLUSTER_NAME"
    log "  Region: $REGION"
    log "  Namespace: $NAMESPACE"
    if [ -n "$SERVICE_NAME" ]; then
        log "  Service: $SERVICE_NAME"
    else
        log "  Service: (auto-detect)"
    fi
    echo ""
    
    # Check if kubectl context is correct
    CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ ! $CURRENT_CONTEXT == *"$CLUSTER_NAME"* ]]; then
        warn "Current kubectl context: $CURRENT_CONTEXT"
        warn "Expected context for cluster: $CLUSTER_NAME"
        log "Make sure you're connected to the right cluster"
    fi
    
    # Find service if not specified
    if [ -z "$SERVICE_NAME" ]; then
        find_documentdb_service
    fi
    
    # Check if service exists
    if ! kubectl get service $SERVICE_NAME -n $NAMESPACE &>/dev/null; then
        error "Service $SERVICE_NAME not found in namespace $NAMESPACE"
    fi
    
    success "Found service: $SERVICE_NAME"
    
    # Check current annotations
    if check_service_annotations; then
        log "Service already configured correctly"
    else
        # Patch the service
        patch_service
        
        # Wait a moment for the patch to take effect
        sleep 10
        
        # Check the result
        check_service_annotations || warn "Annotations may not have been applied correctly"
    fi
    
    # Wait for external IP
    wait_for_external_ip
    
    # Check pod status
    check_pod_status
    
    echo ""
    echo "======================================="
    echo "           Patch Complete"
    echo "======================================="
    echo ""
    success "DocumentDB service has been patched with LoadBalancer annotations"
    log "Monitor the DocumentDB pods with:"
    log "  kubectl get pods -n $NAMESPACE -w"
    log ""
    log "Check service status with:"
    log "  kubectl get service $SERVICE_NAME -n $NAMESPACE"
    log ""
    log "Once external IP is assigned, DocumentDB should be accessible via LoadBalancer"
}

# Run main function
main "$@"