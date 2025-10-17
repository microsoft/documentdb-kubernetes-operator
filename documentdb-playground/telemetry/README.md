# DocumentDB Multi-Tenant Telemetry Setup

This directory contains scripts to set up complete multi-tenant telemetry infrastructure for DocumentDB on Azure Kubernetes Service (AKS) with namespace-based isolation and dedicated monitoring stacks per team.

## Prerequisites

- Azure CLI installed and configured
- kubectl installed
- Helm installed
- jq installed (for JSON parsing)
- An active Azure subscription
- Existing AKS cluster with DocumentDB Operator installed

## Scripts Overview

### deploy-multi-tenant-telemetry.sh

**Primary deployment script** that sets up complete multi-tenant infrastructure:
- Creates isolated namespaces for teams (sales-namespace, accounts-namespace)
- Deploys DocumentDB clusters per team with proper CNPG configuration
- Sets up dedicated OpenTelemetry Collectors with CPU/memory monitoring
- Installs separate Prometheus and Grafana instances per team
- Configures proper RBAC and service accounts

**Usage:**
```bash
# Deploy complete multi-tenant stack
./deploy-multi-tenant-telemetry.sh

# Deploy only DocumentDB clusters
./deploy-multi-tenant-telemetry.sh --documentdb-only

# Deploy only telemetry stack
./deploy-multi-tenant-telemetry.sh --telemetry-only

# Skip waiting for deployments (for status checking)
./deploy-multi-tenant-telemetry.sh --skip-wait
```

### setup-grafana-dashboards.sh

**Automated dashboard creation** that programmatically sets up monitoring dashboards:
- Creates comprehensive CPU and Memory monitoring dashboards
- Configures namespace-specific metric filtering
- Includes pod count and resource utilization metrics
- Uses Grafana API for automated deployment

**Usage:**
```bash
# Create dashboard for sales team
./setup-grafana-dashboards.sh sales-namespace

# Create dashboard for accounts team
./setup-grafana-dashboards.sh accounts-namespace
```

### delete-multi-tenant-telemetry.sh

**Application cleanup script** that removes multi-tenant applications while preserving infrastructure:
- Deletes DocumentDB clusters per team
- Removes OpenTelemetry collectors 
- Cleans up Prometheus and Grafana monitoring stacks
- Deletes team namespaces and associated resources

**Usage:**
```bash
# Delete everything (applications only, keeps infrastructure)
./delete-multi-tenant-telemetry.sh --delete-all

# Delete only DocumentDB clusters
./delete-multi-tenant-telemetry.sh --delete-documentdb

# Delete only monitoring (Prometheus/Grafana)
./delete-multi-tenant-telemetry.sh --delete-monitoring

# Delete with no confirmation prompts
./delete-multi-tenant-telemetry.sh --delete-all --force
```

### Infrastructure Management Scripts

#### create-cluster.sh
**Infrastructure setup** - Creates AKS cluster and operators only:
```bash
# Create cluster + DocumentDB operator + OpenTelemetry operator
./create-cluster.sh --install-all

# Create cluster only
./create-cluster.sh

# Install operators on existing cluster
./create-cluster.sh --install-operator
```

#### delete-cluster.sh
**Infrastructure cleanup** - Removes cluster and all Azure resources:
```bash
# Delete entire AKS cluster and Azure resources
./delete-cluster.sh --delete-all

# Delete only cluster (keeps resource group)
./delete-cluster.sh --delete-cluster
```

## Script Organization

### Infrastructure vs Applications

Our scripts are organized with **clean separation of concerns**:

| **Infrastructure Scripts** | **Application Scripts** |
|---------------------------|-------------------------|
| `create-cluster.sh` | `deploy-multi-tenant-telemetry.sh` |
| `delete-cluster.sh` | `delete-multi-tenant-telemetry.sh` |
| | `setup-grafana-dashboards.sh` |

**Infrastructure Scripts** manage:
- ✅ AKS cluster creation/deletion
- ✅ Azure resource management
- ✅ DocumentDB operator installation
- ✅ OpenTelemetry operator installation
- ✅ Core platform components (cert-manager, CSI drivers)

**Application Scripts** manage:
- 📦 DocumentDB cluster deployments per team
- 🔧 OpenTelemetry collector configurations
- 📊 Monitoring stacks (Prometheus, Grafana)
- 🏠 Team namespaces and application resources

### Benefits of This Approach

- **🔄 Reusable Infrastructure**: Create cluster once, deploy multiple application stacks
- **💰 Cost Optimization**: Delete applications without losing cluster setup
- **🔧 Independent Updates**: Update monitoring without touching infrastructure
- **👥 Team Isolation**: Each team can manage their own application stack
- **🚀 Faster Iterations**: Deploy/destroy applications in seconds, not minutes

## Architecture Overview

### Multi-Tenant DocumentDB + Telemetry Stack

Our implementation provides **complete namespace isolation** with dedicated resources per team:

```
┌─── sales-namespace ────────────────────────────┐  ┌─── accounts-namespace ──────────────────────┐
│  • DocumentDB Cluster (documentdb-sales)       │  │  • DocumentDB Cluster (documentdb-accounts) │
│  • OpenTelemetry Collector (sales-focused)     │  │  • OpenTelemetry Collector (accounts-focused)│
│  • Prometheus Server (prometheus-sales)        │  │  • Prometheus Server (prometheus-accounts)   │
│  • Grafana Instance (grafana-sales)            │  │  • Grafana Instance (grafana-accounts)       │
│  • Dedicated RBAC & Service Accounts           │  │  • Dedicated RBAC & Service Accounts         │
└─────────────────────────────────────────────────┘  └──────────────────────────────────────────────┘
```

### What Gets Deployed

#### Per Team/Namespace:
- **DocumentDB Cluster**: CNPG-managed PostgreSQL cluster with proper operator integration
- **OpenTelemetry Collector**: Namespace-scoped metric collection focusing on CPU/Memory
- **Prometheus Server**: Time-series database for storing team-specific metrics  
- **Grafana Instance**: Visualization dashboard with automated dashboard provisioning
- **RBAC Configuration**: Service accounts, cluster roles, and bindings for secure access

#### Shared Components:
- **DocumentDB Operator**: Cluster-wide operator managing all DocumentDB instances
- **OpenTelemetry Operator**: Cluster-wide operator managing collector deployments

## Recommended Workflow

### 1. Infrastructure Setup (One Time)
```bash
# Create AKS cluster with all required operators
cd scripts/
./create-cluster.sh --install-all
```

### 2. Application Deployment (Repeatable)
```bash
# Deploy multi-tenant DocumentDB + monitoring
./deploy-multi-tenant-telemetry.sh

# Create automated dashboards
./setup-grafana-dashboards.sh sales-namespace
./setup-grafana-dashboards.sh accounts-namespace
```

### 3. Access & Monitor
```bash
# Access Grafana dashboards
kubectl port-forward -n sales-namespace svc/grafana-sales 3001:3000 &
kubectl port-forward -n accounts-namespace svc/grafana-accounts 3002:3000 &

# Open in browser: http://localhost:3001 and http://localhost:3002
# Login: admin / admin123
```

### 4. Cleanup Applications (Keep Infrastructure)
```bash
# Remove all applications, keep cluster running
./delete-multi-tenant-telemetry.sh --delete-all
```

### 5. Full Cleanup (When Done)
```bash
# Delete entire Azure infrastructure
./delete-cluster.sh --delete-all
```

## Quick Start Guide

### 1. Deploy Complete Multi-Tenant Stack
```bash
# Deploy DocumentDB clusters + telemetry for both teams
cd scripts/
./deploy-multi-tenant-telemetry.sh
```

### 2. Create Monitoring Dashboards
```bash
# Create automated dashboards for both teams
./setup-grafana-dashboards.sh sales-namespace
./setup-grafana-dashboards.sh accounts-namespace
```

### 3. Access Grafana Dashboards
```bash
# Port-forward to sales Grafana (runs in background)
kubectl port-forward -n sales-namespace svc/grafana-sales 3001:3000 > /dev/null 2>&1 &

# Port-forward to accounts Grafana (runs in background)
kubectl port-forward -n accounts-namespace svc/grafana-accounts 3002:3000 > /dev/null 2>&1 &

# Access dashboards in browser:
# Sales Team: http://localhost:3001 
# Accounts Team: http://localhost:3002
# Login: admin / admin123
```

## Monitoring Capabilities

### Metrics Collected (CPU & Memory Focus)
- **container_cpu_usage_seconds_total**: CPU usage per container
- **container_memory_working_set_bytes**: Memory usage per container  
- **container_spec_memory_limit_bytes**: Memory limits per container
- **Pod count and status metrics**

### Dashboard Features
- **CPU Usage by Container**: Real-time CPU utilization with 5-minute rate calculation
- **Memory Usage by Container**: Memory consumption in MB per container
- **Memory Usage Percentage**: Memory usage as percentage of configured limits
- **Pod Count Monitoring**: Number of active pods per namespace

### Namespace Isolation
Each OpenTelemetry collector is configured with strict namespace filtering:
```yaml
metric_relabel_configs:
  - source_labels: [namespace]
    regex: '^(sales-namespace)$'  # Only sales-namespace metrics
    action: keep
```

## Advanced Usage

### Deployment Options
```bash
# Deploy only DocumentDB clusters (skip telemetry)
./deploy-multi-tenant-telemetry.sh --documentdb-only

# Deploy only telemetry stack (skip DocumentDB)  
./deploy-multi-tenant-telemetry.sh --telemetry-only

# Check deployment status without waiting
./deploy-multi-tenant-telemetry.sh --skip-wait
```

### Accessing Different Components
```bash
# Check DocumentDB cluster status
kubectl get clusters -n sales-namespace
kubectl get clusters -n accounts-namespace

# View OpenTelemetry collector logs
kubectl logs -n sales-namespace -l app.kubernetes.io/name=opentelemetry-collector

# Access Prometheus directly
kubectl port-forward -n sales-namespace svc/prometheus-sales-server 9090:80
```

### Troubleshooting
```bash
# Check all pods status
kubectl get pods -n sales-namespace
kubectl get pods -n accounts-namespace

# View collector configuration
kubectl get otelcol -n sales-namespace otel-collector-sales -o yaml

# Check metric collection
kubectl logs -n sales-namespace deployment/otel-collector-sales
```

## Cost Management

**Important**: This setup creates dedicated resources per team. Monitor costs and clean up when testing is complete:

```bash
# Clean up multi-tenant resources
kubectl delete namespace sales-namespace accounts-namespace

# Or use legacy cleanup (if applicable)
./delete-cluster.sh
```