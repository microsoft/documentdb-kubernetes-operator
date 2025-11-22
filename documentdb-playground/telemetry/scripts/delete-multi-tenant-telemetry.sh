#!/bin/bash

# Multi-Tenant DocumentDB + Telemetry Cleanup Script
# This script removes all multi-tenant DocumentDB applications and monitoring stack

set -e

# Configuration
TEAMS=("sales" "accounts")
NAMESPACES=("sales-namespace" "accounts-namespace")

# Cleanup scope flags  
DELETE_DOCUMENTDB="${DELETE_DOCUMENTDB:-false}"
DELETE_COLLECTORS="${DELETE_COLLECTORS:-false}"
DELETE_MONITORING="${DELETE_MONITORING:-false}" 
DELETE_NAMESPACES="${DELETE_NAMESPACES:-false}"
DELETE_ALL="${DELETE_ALL:-false}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --delete-documentdb)
            DELETE_DOCUMENTDB="true"
            shift
            ;;
        --delete-collectors)
            DELETE_COLLECTORS="true"  
            shift
            ;;
        --delete-monitoring)
            DELETE_MONITORING="true"
            shift
            ;;
        --delete-namespaces)
            DELETE_NAMESPACES="true"
            shift
            ;;
        --delete-all)
            DELETE_ALL="true"
            DELETE_DOCUMENTDB="true"
            DELETE_COLLECTORS="true"
            DELETE_MONITORING="true"
            DELETE_NAMESPACES="true"
            shift
            ;;
        --force)
            FORCE_DELETE="true"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Multi-tenant DocumentDB and telemetry cleanup script"
            echo ""
            echo "Options:"
            echo "  --delete-documentdb     Delete DocumentDB clusters only"
            echo "  --delete-collectors     Delete OpenTelemetry collectors only"
            echo "  --delete-monitoring     Delete Prometheus/Grafana monitoring only"
            echo "  --delete-namespaces     Delete team namespaces (includes all above)"
            echo "  --delete-all            Delete everything (DocumentDB + collectors + monitoring + namespaces)"
            echo "  --force                 Skip confirmation prompts"
            echo "  -h, --help             Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --delete-all                    # Remove everything"
            echo "  $0 --delete-documentdb             # Remove only DocumentDB clusters"  
            echo "  $0 --delete-monitoring             # Remove only Prometheus/Grafana"
            echo "  $0 --delete-all --force            # Remove everything without confirmation"
            echo ""
            echo "Affected namespaces: ${NAMESPACES[*]}"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
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

# Logging functions
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
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl not found. Cannot proceed with cleanup."
    fi
    
    # Check cluster access
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot access Kubernetes cluster. Please check your kubectl configuration."
    fi
    
    # Check Helm
    if ! command -v helm &> /dev/null; then
        warn "Helm not found. Some monitoring cleanup may require manual intervention."
    fi
    
    success "Prerequisites met"
}

# Confirmation prompt
confirm_deletion() {
    if [ "$FORCE_DELETE" == "true" ]; then
        return 0
    fi
    
    echo ""
    echo "⚠️  WARNING: This will permanently delete the following multi-tenant resources:"
    echo ""
    
    if [ "$DELETE_DOCUMENTDB" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        echo "📦 DocumentDB Clusters:"
        for team in "${TEAMS[@]}"; do
            echo "  - documentdb-$team (in ${team}-namespace)"
        done
    fi
    
    if [ "$DELETE_COLLECTORS" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        echo "🔧 OpenTelemetry Collectors:"
        for team in "${TEAMS[@]}"; do
            echo "  - documentdb-${team}-collector (in ${team}-namespace)"
        done
    fi
    
    if [ "$DELETE_MONITORING" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        echo "📊 Monitoring Stacks:"
        for team in "${TEAMS[@]}"; do
            echo "  - prometheus-$team (Helm release)"
            echo "  - grafana-$team (Helm release)"
        done
    fi
    
    if [ "$DELETE_NAMESPACES" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        echo "🏠 Namespaces:"
        for ns in "${NAMESPACES[@]}"; do
            echo "  - $ns (and ALL resources within it)"
        done
    fi
    
    echo ""
    echo "💡 This will NOT affect:"
    echo "  - AKS cluster infrastructure"
    echo "  - DocumentDB operator"
    echo "  - OpenTelemetry operator" 
    echo "  - Other namespaces"
    echo ""
    
    read -p "Are you sure you want to proceed? (yes/no): " -r
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        log "Operation cancelled by user"
        exit 0
    fi
}

# Delete DocumentDB clusters
delete_documentdb_clusters() {
    log "Deleting DocumentDB clusters..."
    
    for i in "${!TEAMS[@]}"; do
        team="${TEAMS[$i]}"
        namespace="${NAMESPACES[$i]}"
        
        log "Deleting DocumentDB cluster for team: $team"
        
        # Delete DocumentDB cluster
        kubectl delete documentdb documentdb-$team -n $namespace --ignore-not-found=true || warn "DocumentDB cluster for $team not found or failed to delete"
        
        # Wait for cluster to be fully deleted
        log "Waiting for DocumentDB cluster $team to be fully deleted..."
        timeout=120
        while kubectl get documentdb documentdb-$team -n $namespace &> /dev/null && [ $timeout -gt 0 ]; do
            echo -n "."
            sleep 2
            timeout=$((timeout - 2))
        done
        echo ""
        
        if [ $timeout -le 0 ]; then
            warn "Timeout waiting for DocumentDB cluster $team to be deleted"
        else
            success "DocumentDB cluster $team deleted successfully"
        fi
        
        # Delete secrets and configmaps
        kubectl delete secret documentdb-credentials -n $namespace --ignore-not-found=true || true
        kubectl delete configmap --all -n $namespace --ignore-not-found=true || true
    done
    
    success "DocumentDB clusters cleanup completed"
}

# Delete OpenTelemetry collectors
delete_otel_collectors() {
    log "Deleting OpenTelemetry collectors..."
    
    for i in "${!TEAMS[@]}"; do
        team="${TEAMS[$i]}"
        namespace="${NAMESPACES[$i]}"
        
        log "Deleting OpenTelemetry collector for team: $team"
        
        # Delete OpenTelemetry collector
        kubectl delete otelcol documentdb-${team}-collector -n $namespace --ignore-not-found=true || warn "OpenTelemetry collector for $team not found"
        
        # Delete collector service account and RBAC
        kubectl delete serviceaccount otel-collector-$team -n $namespace --ignore-not-found=true || true
        kubectl delete clusterrolebinding otel-collector-$team --ignore-not-found=true || true
    done
    
    success "OpenTelemetry collectors cleanup completed"
}

# Delete monitoring stack (Prometheus & Grafana)
delete_monitoring_stack() {
    log "Deleting monitoring stacks..."
    
    if ! command -v helm &> /dev/null; then
        error "Helm is required to delete monitoring stack. Please install Helm or delete manually."
    fi
    
    for team in "${TEAMS[@]}"; do
        namespace="${team}-namespace"
        
        log "Deleting monitoring stack for team: $team"
        
        # Delete Grafana
        log "Deleting Grafana for $team..."
        helm uninstall grafana-$team -n $namespace --ignore-not-found 2>/dev/null || warn "Grafana release for $team not found"
        
        # Delete Prometheus  
        log "Deleting Prometheus for $team..."
        helm uninstall prometheus-$team -n $namespace --ignore-not-found 2>/dev/null || warn "Prometheus release for $team not found"
        
        # Wait for PVCs to be cleaned up (they may have finalizers)
        log "Waiting for persistent volumes to be cleaned up..."
        sleep 5
        
        # Force delete any remaining PVCs if they exist
        kubectl delete pvc --all -n $namespace --ignore-not-found=true || true
    done
    
    success "Monitoring stacks cleanup completed"
}

# Delete team namespaces
delete_team_namespaces() {
    log "Deleting team namespaces..."
    
    for namespace in "${NAMESPACES[@]}"; do
        log "Deleting namespace: $namespace"
        
        # Delete namespace (this will delete all resources within it)
        kubectl delete namespace $namespace --ignore-not-found=true || warn "Failed to delete namespace $namespace"
        
        # Wait for namespace to be fully deleted
        log "Waiting for namespace $namespace to be fully deleted..."
        timeout=120
        while kubectl get namespace $namespace &> /dev/null && [ $timeout -gt 0 ]; do
            echo -n "."
            sleep 2
            timeout=$((timeout - 2))
        done
        echo ""
        
        if [ $timeout -le 0 ]; then
            warn "Timeout waiting for namespace $namespace to be deleted"
        else
            success "Namespace $namespace deleted successfully"
        fi
    done
    
    success "Team namespaces cleanup completed"
}

# Clean up cluster-wide resources specific to multi-tenant setup
cleanup_cluster_resources() {
    log "Cleaning up cluster-wide multi-tenant resources..."
    
    # Delete cluster roles and bindings for each team
    for team in "${TEAMS[@]}"; do
        kubectl delete clusterrole otel-collector-$team --ignore-not-found=true || true
        kubectl delete clusterrolebinding otel-collector-$team --ignore-not-found=true || true
    done
    
    success "Cluster-wide resources cleaned up"
}

# Main execution function
main() {
    log "Starting multi-tenant DocumentDB + telemetry cleanup..."
    
    check_prerequisites
    
    # If no specific flags are set, show help
    if [ "$DELETE_DOCUMENTDB" != "true" ] && [ "$DELETE_COLLECTORS" != "true" ] && [ "$DELETE_MONITORING" != "true" ] && [ "$DELETE_NAMESPACES" != "true" ] && [ "$DELETE_ALL" != "true" ]; then
        warn "No cleanup scope specified. Use --help to see available options."
        echo ""
        echo "Quick options:"
        echo "  --delete-all        Delete everything"
        echo "  --delete-documentdb Delete DocumentDB clusters only"
        echo "  --help              Show full help"
        exit 1
    fi
    
    confirm_deletion
    
    # Execute cleanup in proper order
    if [ "$DELETE_DOCUMENTDB" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        delete_documentdb_clusters
    fi
    
    if [ "$DELETE_COLLECTORS" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        delete_otel_collectors
    fi
    
    if [ "$DELETE_MONITORING" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        delete_monitoring_stack
    fi
    
    if [ "$DELETE_NAMESPACES" == "true" ] || [ "$DELETE_ALL" == "true" ]; then
        delete_team_namespaces
    else
        # Clean up cluster resources even if not deleting namespaces
        cleanup_cluster_resources
    fi
    
    # Summary
    echo ""
    echo "=================================================="
    echo "🎉 MULTI-TENANT CLEANUP COMPLETE!"
    echo "=================================================="
    echo ""
    echo "✅ Cleanup completed successfully"
    echo ""
    echo "💡 What was cleaned up:"
    [ "$DELETE_DOCUMENTDB" == "true" ] || [ "$DELETE_ALL" == "true" ] && echo "  - DocumentDB clusters for teams: ${TEAMS[*]}"
    [ "$DELETE_COLLECTORS" == "true" ] || [ "$DELETE_ALL" == "true" ] && echo "  - OpenTelemetry collectors for teams: ${TEAMS[*]}"
    [ "$DELETE_MONITORING" == "true" ] || [ "$DELETE_ALL" == "true" ] && echo "  - Prometheus/Grafana monitoring stacks"
    [ "$DELETE_NAMESPACES" == "true" ] || [ "$DELETE_ALL" == "true" ] && echo "  - Team namespaces: ${NAMESPACES[*]}"
    echo ""
    echo "🏗️  Infrastructure still available:"
    echo "  - AKS cluster (use delete-cluster.sh to remove)"
    echo "  - DocumentDB operator"
    echo "  - OpenTelemetry operator"
    echo ""
    echo "🚀 Ready for new multi-tenant deployments!"
    echo "   Use: ./deploy-multi-tenant-telemetry.sh"
}

# Run main function
main "$@"