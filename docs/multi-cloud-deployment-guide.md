# Multi-Cloud DocumentDB Deployment Guide

This guide provides step-by-step instructions for setting up a multi-cloud
deployment of DocumentDB using KubeFleet to manage clusters across
clouds. This setup enables high availability and disaster recovery.
We assume the use of an AKS cluster and an on-prem Kubernetes cluster
that have network access to one another.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [Setting Up the Hub Cluster](#setting-up-the-hub-cluster)
4. [Adding Clusters to Fleet](#adding-clusters-to-fleet)
5. [Installing Operators and Dependencies](#installing-operators-and-dependencies)
6. [Deploying DocumentDB Operator to Fleet](#deploying-documentdb-operator-to-fleet)
7. [Setting Up Replication](#setting-up-replication)
8. [Testing and Verification](#testing-and-verification)
9. [Failover Procedures](#failover-procedures)

## Prerequisites

- Azure account
- Azure CLI installed and configured with appropriate permissions
- kubectl installed
- helm installed
- Git client
- `kazure` alias like `alias kazure="kubectl --kubeconfig=/path/to/azure-kubeconfig"`
- The same for `konprem`

## Architecture Overview

This multi-cloud deployment uses KubeFleet to manage DocumentDB instances across different cloud providers:

- **Hub Cluster**: Central control plane for managing all member clusters
- **Member Clusters**: Clusters in different cloud environments (Azure, On-prem)
- **DocumentDB Operator**: Custom operator for DocumentDB deployments
- **Fleet Networking**: Enables communication between clusters

## Setting Up the Hub Cluster

The hub cluster serves as the central controller for managing the member clusters, find setup instructions here:
https://learn.microsoft.com/en-us/azure/kubernetes-fleet/quickstart-create-fleet-and-members?tabs=without-hub-cluster

## Adding Clusters to Fleet

### Adding AKS cluster to the fleet

Adding an AKS cluster to the fleet is very simple with the Azure portal: 
https://learn.microsoft.com/en-us/azure/kubernetes-fleet/quickstart-create-fleet-and-members-portal

### Adding other Cluster to Fleet

See also the guide here: 
https://github.com/Azure/fleet/blob/main/docs/tutorials/Azure/JoinOnPremClustersToFleet.md

```bash
# Add the hub to your kubectl config file
az fleet get-credentials --resource-group fleet-resource-group --name fleet-hub-name

# This needs to match the member cluster name in your kubectl config file
clusterName="your-on-prem-cluster-name"

git clone https://github.com/kubefleet-dev/kubefleet.git
cd kubefleet
./hack/membership/joinMC.sh v0.14.8 hub $clusterName
cd ..

# Wait until the cluster shows the correct number of nodes, usually about a minute

git clone https://github.com/Azure/fleet-networking
cd fleet-networking
./hack/membership/joinMC.sh v0.14.8 v0.3.8 hub $clusterName
cd ..
```

These commands also will work to add clusters from other cloud providers 

## Installing Operators and Dependencies

1. Install cert-manager on each cluster:

```bash
# Install on primary
helm --kubeconfig /primary/kubeconfig/ repo add jetstack https://charts.jetstack.io
helm --kubeconfig /primary/kubeconfig/ repo update
helm --kubeconfig /primary/kubeconfig/ install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true

# Install on replica
helm --kubeconfig /replica/kubeconfig/ repo add jetstack https://charts.jetstack.io
helm --kubeconfig /replica/kubeconfig/ repo update
helm --kubeconfig /replica/kubeconfig/ install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true
```

2. Install the DocumentDB operator on the hub:

```bash
helm install documentdb-operator oci://ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-operator --version 0.0.1 --namespace documentdb-operator --create-namespace
```

3. Deploy certificates on each cluster:

```bash
cat <<EOF > certs.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: cnpg-system
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: helloworld-client
  namespace: cnpg-system
spec:
  commonName: helloworld-client
  duration: 2160h
  isCA: false
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: selfsigned-issuer
  renewBefore: 360h
  secretName: helloworld-client-tls
  usages:
  - client auth
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: helloworld-server
  namespace: cnpg-system
spec:
  commonName: hello-world
  dnsNames:
  - hello-world
  duration: 2160h
  isCA: false
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: selfsigned-issuer
  renewBefore: 360h
  secretName: helloworld-server-tls
  usages:
  - server auth
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: cnpg-system
spec:
  selfSigned: {}
EOF

kazure apply -f certs.yaml
konprem apply -f certs.yaml
```

## Deploying DocumentDB Operator to fleet

```bash
cat <<EOF > documentdb-base.yaml
apiVersion: placement.kubernetes-fleet.io/v1beta1
kind: ClusterResourcePlacement
metadata:
  name: documentdb-base
spec:
  resourceSelectors:
    - group: ""
      version: v1
      kind: Namespace
      name: documentdb-operator
    - group: ""
      version: v1
      kind: Namespace
      name: cnpg-system
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: documentdbs.db.microsoft.com
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: clusters.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: databases.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: publications.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: subscriptions.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: poolers.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: backups.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: clusterimagecatalogs.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: scheduledbackups.postgresql.cnpg.io
    - group: "apiextensions.k8s.io"
      version: v1
      kind: CustomResourceDefinition
      name: imagecatalogs.postgresql.cnpg.io
    # RBAC roles and bindings
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: manager-role
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRoleBinding
      name: manager-rolebinding
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: documentdb-operator-cluster-role
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: documentdb-operator-admin-role
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: documentdb-operator-editor-role
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: documentdb-operator-viewer-role
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRoleBinding
      name: documentdb-operator-cluster-rolebinding
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: cnpg-operator-cloudnative-pg
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRoleBinding
      name: cnpg-operator-cloudnative-pg
    - group: "rbac.authorization.k8s.io"
      version: v1
      kind: ClusterRole
      name: cnpg-operator-cloudnative-pg-edit
EOF

kubectl apply -f ./documentdb-base.yaml
```

## Setting Up Replication

Physical replication provides high availability and disaster recovery capabilities across clusters.

1. Create configuration maps to identify clusters:

```bash
konprem create configmap cluster-name -n kube-system --from-literal=name=replica-cluster
kazure create configmap cluster-name -n kube-system --from-literal=name=primary-cluster
```

OR

```bash
cat <<EOF > primary-name.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-name
  namespace: kube-system
data:
  name: "primary-cluster"
EOF

cat <<EOF > replica-name.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-name
  namespace: kube-system
data:
  name: "replica-cluster"
EOF

kazure apply -f ./primary-name.yaml
konprem apply -f ./replica-name.yaml
```


2. Apply the DocumentDB resource configuration:

```bash
cat <<EOF > documentdb-resource.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: documentdb-preview-ns

---

apiVersion: db.microsoft.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 10Gi
  clusterReplication:
    fleetEnabled: true
    primary: primary-cluster
    clusterList:
      - primary-cluster
      - replica-cluster

---

apiVersion: placement.kubernetes-fleet.io/v1beta1
kind: ClusterResourcePlacement
metadata:
  name: documentdb-crp
spec:
  resourceSelectors:
    - group: ""
      version: v1
      kind: Namespace
      name: documentdb-preview-ns
  policy:
    placementType: PickAll
  strategy:
    type: RollingUpdate
EOF

kubectl apply -f ./documentdb-resource.yaml
```


4. To perform a failover:

```bash
kubectl patch documentdb documentdb-preview -n documentdb-preview-ns \
  --type='json' -p='[
  {"op": "replace", "path": "/spec/clusterReplication/primary", "value":"replica-cluster"},
  {"op": "replace", "path": "/spec/clusterReplication/clusterList", "value":["replica-cluster"]}
  ]'
```

## Testing and Verification

1. Test connection to DocumentDB:

```bash
# Get the service IP from primary (azure)
service_ip=$(kazure get service documentdb-service-documentdb-preview -n documentdb-preview-ns -o jsonpath="{.status.loadBalancer.ingress[0].ip}")

# Connect using mongosh
mongosh $service_ip:10260 -u default_user -p Admin100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates

# Create a test collection and document
use test
db.testCollection.insertOne({ name: "Test Document", value: 1 })
db.testCollection.find()
```

2. For replication testing, verify data is correctly replicated:
   - Insert data on the primary
   - Verify it appears on the replica

## Failover Procedures

### Physical Replication Failover

To initiate a failover from the primary to a replica cluster:

```bash
kubectl patch documentdb documentdb-preview -n documentdb-preview-ns \
  --type='json' -p='[
  {"op": "replace", "path": "/spec/clusterReplication/primary", "value":"replica-cluster"},
  {"op": "replace", "path": "/spec/clusterReplication/clusterList", "value":["replica-cluster"]}
  ]'
```

---

**Note**: This guide assumes a basic understanding of Kubernetes, container orchestration, and distributed database concepts. Adjust resource sizes, node counts, and other parameters according to your production requirements.
