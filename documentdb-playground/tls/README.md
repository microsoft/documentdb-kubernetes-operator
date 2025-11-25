# DocumentDB Gateway TLS Setup

This directory contains automated scripts for setting up DocumentDB with Gateway TLS support on Azure Kubernetes Service (AKS).

## Overview

The DocumentDB Kubernetes Operator supports multiple TLS modes for gateway components:
- **SelfSigned**: Operator automatically provisions self-signed certificates via cert-manager
- **Provided**: Use externally provided certificates (e.g., from Azure Key Vault)
- **CertManager**: Use cert-manager with custom issuers

This setup automates the entire process, including:
- AKS cluster creation with all required addons
- cert-manager installation
- Azure Key Vault setup (for Provided mode)
- Secrets Store CSI driver configuration
- DocumentDB operator deployment
- TLS certificate provisioning and validation

## Documentation

- 📖 **[E2E-TESTING.md](E2E-TESTING.md)** - Comprehensive automated and manual E2E testing guide
- 📘 **[MANUAL-PROVIDED-MODE-SETUP.md](MANUAL-PROVIDED-MODE-SETUP.md)** - Detailed step-by-step manual guide for Provided TLS mode with Azure Key Vault

## Quick Start

### Prerequisites

Before running the scripts, ensure you have the following tools installed:
- `az` (Azure CLI) - [Installation guide](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli)
- `kubectl` - [Installation guide](https://kubernetes.io/docs/tasks/tools/)
- `helm` - [Installation guide](https://helm.sh/docs/intro/install/)
- `mongosh` (MongoDB Shell) - [Installation guide](https://www.mongodb.com/docs/mongodb-shell/install/)
- `openssl` - Usually pre-installed on macOS/Linux

You must also have:
- An Azure subscription with Owner permissions
- Authenticated Azure CLI session (`az login`)

### Create Complete TLS-Enabled Cluster

Run the following command to create a complete AKS cluster with DocumentDB and TLS enabled:

```bash
./scripts/create-cluster.sh \
  --suffix myusername \
  --subscription-id <your-azure-subscription-id>
```

**What this does:**
1. Creates an AKS cluster with necessary addons (cert-manager, CSI driver)
2. Deploys the DocumentDB operator
3. Sets up both SelfSigned and Provided TLS modes
4. Validates TLS connectivity
5. Provides connection strings for testing

**Default Configuration:**
- **Location**: `eastus2`
- **Resource Group**: `guanzhou-<suffix>-rg`
- **AKS Cluster**: `guanzhou-<suffix>`
- **Key Vault**: `ddb-issuer-<suffix>`
- **Namespace**: `documentdb-preview-ns`
- **DocumentDB Name**: `documentdb-preview`

### Customize Your Setup

You can override defaults with additional flags:

```bash
./scripts/create-cluster.sh \
  --suffix myusername \
  --subscription-id <your-subscription-id> \
  --location westus2 \
  --resource-group my-rg \
  --aks-name my-aks-cluster \
  --keyvault my-keyvault \
  --namespace my-namespace \
  --docdb-name my-documentdb
```

### Use Existing Cluster

If you already have an AKS cluster and want to add DocumentDB with TLS:

```bash
./scripts/create-cluster.sh \
  --suffix myusername \
  --subscription-id <your-subscription-id> \
  --skip-cluster
```

This will skip cluster creation and install DocumentDB components on the current kubectl context.

### Clean Up Resources

To delete all resources created by the script:

```bash
./scripts/delete-cluster.sh \
  --suffix myusername \
  --subscription-id <your-subscription-id> \
  --all
```

**Options:**
- `--all`: Delete everything (AKS cluster, resource group, Key Vault)
- `--keep-cluster`: Delete only DocumentDB resources, keep the cluster
- `--keep-keyvault`: Delete cluster but preserve Key Vault

### Verify TLS Setup

After the cluster is created, verify TLS connectivity:

```bash
# Get DocumentDB status
kubectl get documentdb documentdb-preview -n documentdb-preview-ns

# Check TLS status specifically
kubectl get documentdb documentdb-preview -n documentdb-preview-ns -o jsonpath='{.status.tls}' | jq

# Connect using mongosh with TLS
mongosh "mongodb://$(kubectl get secret documentdb-credentials -n documentdb-preview-ns -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret documentdb-credentials -n documentdb-preview-ns -o jsonpath='{.data.password}' | base64 -d)@<EXTERNAL-IP>:10260/?tls=true&tlsAllowInvalidCertificates=true"
```

## Scripts Directory

All TLS setup and testing scripts are located in the `scripts/` directory.

## 🚀 Main Scripts

### create-cluster.sh

**Main entry point** - Creates complete TLS-enabled DocumentDB cluster with simplified interface.

**Common Options:**
- `--suffix <value>`: Unique identifier for resource names (required)
- `--subscription-id <id>`: Azure subscription ID (required)
- `--location <region>`: Azure region (default: eastus2)
- `--resource-group <name>`: Resource group name (default: guanzhou-<suffix>-rg)
- `--aks-name <name>`: AKS cluster name (default: guanzhou-<suffix>)
- `--keyvault <name>`: Key Vault name (default: ddb-issuer-<suffix>)
- `--namespace <name>`: Kubernetes namespace (default: documentdb-preview-ns)
- `--docdb-name <name>`: DocumentDB resource name (default: documentdb-preview)
- `--skip-cluster`: Skip AKS cluster creation
- `--github-username <user>`: GitHub username for operator images (optional)
- `--github-token <token>`: GitHub token for private registries (optional)
- `--help`: Show usage information

**Examples:**

```bash
# Minimal setup
./scripts/create-cluster.sh --suffix demo --subscription-id abc123

# Production setup with custom names
./scripts/create-cluster.sh \
  --suffix prod \
  --subscription-id abc123 \
  --location westus2 \
  --resource-group documentdb-prod-rg \
  --aks-name documentdb-prod-aks

# Development setup on existing cluster
./scripts/create-cluster.sh \
  --suffix dev \
  --subscription-id abc123 \
  --skip-cluster
```

### delete-cluster.sh

**Cleanup script** - Removes resources with flexible cleanup options.

**Options:**
- `--suffix <value>`: Suffix used during creation (required)
- `--subscription-id <id>`: Azure subscription ID (required)
- `--all`: Delete all resources including cluster and resource group
- `--keep-cluster`: Delete DocumentDB resources but keep the AKS cluster
- `--keep-keyvault`: Delete cluster but preserve Key Vault
- `--resource-group <name>`: Override resource group name
- `--aks-name <name>`: Override AKS cluster name
- `--keyvault <name>`: Override Key Vault name
- `--namespace <name>`: Override namespace
- `--help`: Show usage information

**Examples:**

```bash
# Delete everything
./scripts/delete-cluster.sh --suffix demo --subscription-id abc123 --all

# Delete only DocumentDB, keep cluster for reuse
./scripts/delete-cluster.sh --suffix demo --subscription-id abc123 --keep-cluster

# Delete cluster but preserve Key Vault data
./scripts/delete-cluster.sh --suffix demo --subscription-id abc123 --keep-keyvault
```

## 🔧 Core Scripts

### gateway-tls-e2e.sh

**Comprehensive E2E script** - Handles full lifecycle from infrastructure to validation.
- Creates AKS cluster with all prerequisites
- Deploys operator and DocumentDB
- Tests both SelfSigned and Provided TLS modes
- Validates connectivity

**Used by**: `create-cluster.sh` (wrapper for simplified interface)

**Direct usage** (for advanced control):
```bash
./scripts/gateway-tls-e2e.sh --suffix test --location eastus2
```

## 🔐 TLS Configuration Scripts

### setup-selfsigned-gateway-tls.sh

**SelfSigned TLS mode** - Configure cert-manager with self-signed issuer (requires existing cluster and operator).

```bash
./scripts/setup-selfsigned-gateway-tls.sh --namespace my-ns --docdb-name my-db
```

### setup-documentdb-akv.sh

**Azure Key Vault setup** - Create and configure AKV for certificates.

```bash
./scripts/setup-documentdb-akv.sh --suffix test --keyvault my-kv --resource-group my-rg
```

### documentdb-provided-mode-setup.sh

**Provided TLS mode** - Configure DocumentDB to use external certificates.

```bash
./scripts/documentdb-provided-mode-setup.sh \
  --namespace my-ns \
  --docdb-name my-db \
  --secret-name my-tls-secret
```

## ✅ Validation Scripts

### tls-connectivity-check.sh

**TLS validation** - Verify TLS configuration and connectivity.

```bash
./scripts/tls-connectivity-check.sh --namespace my-ns --docdb-name my-db
```

## 📋 Script Workflow

### Standard E2E Test Flow
```
create-cluster.sh
    ↓
gateway-tls-e2e.sh
    ├── Create AKS cluster
    ├── Install cert-manager
    ├── Install CSI driver
    ├── setup-documentdb-akv.sh
    ├── Deploy operator
    ├── setup-selfsigned-gateway-tls.sh
    ├── tls-connectivity-check.sh
    ├── documentdb-provided-mode-setup.sh
    └── tls-connectivity-check.sh
```

### Cleanup Flow
```
delete-cluster.sh
    ├── Delete Kubernetes resources
    ├── Delete namespace
    ├── (Optional) Delete AKS cluster
    └── (Optional) Delete resource group
```

## 🎯 Common Use Cases

### First Time Setup
```bash
./scripts/create-cluster.sh --suffix myname --subscription-id <id>
```

### Using Existing Cluster
```bash
./scripts/create-cluster.sh --suffix myname --subscription-id <id> --skip-cluster
```

### Test SelfSigned Mode Only
```bash
# Assumes cluster and operator exist
./scripts/setup-selfsigned-gateway-tls.sh --namespace my-ns --docdb-name my-db
./scripts/tls-connectivity-check.sh --namespace my-ns --docdb-name my-db
```

### Test Provided Mode Only
```bash
# Setup Key Vault
./scripts/setup-documentdb-akv.sh --suffix test --keyvault my-kv --resource-group my-rg

# Configure DocumentDB
./scripts/documentdb-provided-mode-setup.sh --namespace my-ns --docdb-name my-db --secret-name provided-tls

# Validate
./scripts/tls-connectivity-check.sh --namespace my-ns --docdb-name my-db
```

### Complete Cleanup
```bash
./scripts/delete-cluster.sh --suffix myname --subscription-id <id> --all
```

### Partial Cleanup (Keep Cluster)
```bash
./scripts/delete-cluster.sh --suffix myname --subscription-id <id> --keep-cluster
```

## 🛠️ All Scripts Summary

| Script | Purpose | Duration | Dependencies |
|--------|---------|----------|--------------|
| `create-cluster.sh` | Main entry point (wrapper) | ~25-30 min | None |
| `delete-cluster.sh` | Cleanup | ~5-10 min | None |
| `gateway-tls-e2e.sh` | Full E2E setup (core) | ~25-30 min | Azure CLI, kubectl |
| `setup-selfsigned-gateway-tls.sh` | SelfSigned mode | ~2-3 min | Existing cluster |
| `setup-documentdb-akv.sh` | Key Vault setup | ~3-5 min | Azure CLI |
| `documentdb-provided-mode-setup.sh` | Provided mode | ~2-3 min | Existing cluster |
| `tls-connectivity-check.sh` | Validation | ~1-2 min | mongosh |

## 🔍 Getting Help

Each script supports `--help`:
```bash
./scripts/create-cluster.sh --help
./scripts/delete-cluster.sh --help
```

For detailed E2E testing workflows, see **[E2E-TESTING.md](E2E-TESTING.md)**.

---

## TLS Modes Explained

### SelfSigned Mode

The operator automatically creates:
- A self-signed ClusterIssuer via cert-manager
- A Certificate resource for the gateway
- A Kubernetes secret with the TLS certificate

**Use Case**: Development, testing, or internal environments where self-signed certificates are acceptable.

**Configuration Example:**
```yaml
apiVersion: db.documentdb.com/v1
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  tls:
    gateway:
      mode: SelfSigned
```

### Provided Mode

You provide an existing certificate from an external source (e.g., Azure Key Vault, Let's Encrypt, enterprise CA).

**Use Case**: Production environments with existing PKI infrastructure or certificate management systems.

**Configuration Example:**
```yaml
apiVersion: db.documentdb.com/v1
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  tls:
    gateway:
      mode: Provided
      provided:
        secretName: my-tls-secret
```

### CertManager Mode

Use cert-manager with your own Issuer or ClusterIssuer (e.g., Let's Encrypt, Venafi).

**Use Case**: Production environments with automated certificate renewal requirements.

**Configuration Example:**
```yaml
apiVersion: db.documentdb.com/v1
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  tls:
    gateway:
      mode: CertManager
      certManager:
        issuerRef:
          name: letsencrypt-prod
          kind: ClusterIssuer
        dnsNames:
          - documentdb.example.com
```

## Troubleshooting

### Check TLS Certificate Status

```bash
# Check DocumentDB TLS status
kubectl get documentdb documentdb-preview -n documentdb-preview-ns -o yaml | grep -A 10 "tls:"

# Check cert-manager certificates
kubectl get certificates -n documentdb-preview-ns

# Check certificate details
kubectl describe certificate <cert-name> -n documentdb-preview-ns

# Check TLS secret
kubectl get secret <secret-name> -n documentdb-preview-ns -o yaml
```

### Verify Gateway Pod Configuration

```bash
# Check gateway sidecar logs
kubectl logs -n documentdb-preview-ns <pod-name> -c gateway-sidecar

# Verify TLS secret is mounted
kubectl describe pod <pod-name> -n documentdb-preview-ns | grep -A 5 "Mounts:"
```

### Common Issues

**Certificate Not Ready:**
```bash
# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check certificate status
kubectl describe certificate -n documentdb-preview-ns
```

**Azure Key Vault Access Issues:**
```bash
# Verify managed identity binding
kubectl get azureidentitybinding -n documentdb-preview-ns

# Check CSI driver logs
kubectl logs -n kube-system -l app=secrets-store-csi-driver
```

**Connection Issues:**
```bash
# Test TLS connectivity
openssl s_client -connect <external-ip>:10260 -servername documentdb-preview

# Verify service external IP
kubectl get svc -n documentdb-preview-ns
```

## Additional Resources

- [E2E Testing Guide](E2E-TESTING.md) - Comprehensive automated and manual testing procedures
- [Advanced Configuration](../../docs/operator-public-documentation/v1/advanced-configuration/README.md) - Production configurations and best practices
- [DocumentDB Operator Documentation](https://microsoft.github.io/documentdb-kubernetes-operator) - Complete operator documentation
- [cert-manager Documentation](https://cert-manager.io/docs/) - Certificate management
- [Azure Key Vault CSI Driver](https://azure.github.io/secrets-store-csi-driver-provider-azure/) - Azure secrets integration

## Support

For issues or questions:
- Create an [issue](https://github.com/microsoft/documentdb-kubernetes-operator/issues)
- Check [documentation](https://microsoft.github.io/documentdb-kubernetes-operator)
- Review [E2E Testing Guide](E2E-TESTING.md#troubleshooting) for troubleshooting
