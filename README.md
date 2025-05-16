# DocumentDB Kubernetes Operator

The DocumentDB Kubernetes Operator is an open-source project to run and manage [DocumentDB](https://github.com/microsoft/documentdb) on using Kubernetes. `DocumentDB` is the engine powering vCore-based Azure Cosmos DB for MongoDB. It is built on top of PostgreSQL and offers a native implementation of document-oriented NoSQL database, enabling CRUD operations on BSON data types.

# Quick Start

## Prerequisites

- Helm installed
- kubectl installed
- Azure CLI installed (if you are using Azure Kubernetes Service (AKS))

## Quick Test Using the Published Helm Chart

### 1. Install the `documentdb-operator` using the Helm chart.

```sh
helm install documentdb-operator oci://ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-operator --version 0.0.1 --namespace documentdb-operator --create-namespace
```

### 2. Verify that the DocumentDB operator is running in the `documentdb-operator` namespace.

```sh
kubectl get pod -n documentdb-operator
NAME                                  READY   STATUS    RESTARTS   AGE
documentdb-operator-7fc8684bf-9q4nh   1/1     Running   0          18m
```

### 3. Deploy a single-node DocumentDB cluster

The following Kubernetes manifest creates a DocumentDB cluster with a single node and only a primary instance on it. The manifest creates the `documentdb-preview-ns` namespace, a public load balancer service `documentdb-service-documentdb-preview`, and the DocumentDB pod `documentdb-preview-1` in the same namespace.

```sh
cat <<EOF | kubectl apply -f -
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
  documentDBImage: ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-local:16
  resource:
    pvcSize: 10Gi
  publicLoadBalancer:
    enabled: true
EOF
```

### 4. Verify that the necessary services and pods are created.

```sh
kubectl get services -n documentdb-preview-ns
NAME                                    TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)           AGE
documentdb-preview-r                    ClusterIP      10.0.216.38    <none>          5432/TCP          26m
documentdb-preview-ro                   ClusterIP      10.0.31.103    <none>          5432/TCP          26m
documentdb-preview-rw                   ClusterIP      10.0.118.26    <none>          5432/TCP          26m
documentdb-service-documentdb-preview   LoadBalancer   10.0.228.243   52.149.56.216   10260:30312/TCP   27m

kubectl get pods -n documentdb-preview-ns
NAME                   READY   STATUS    RESTARTS   AGE
documentdb-preview-1   2/2     Running   0          26m
```

### 5. Test pushing some dummy documents into your DocumentDB

We have a test Python script that uses a PyMongo client to push a test document into DocumentDB and read it back. Update the script with the external IP of your `documentdb-service-documentdb-preview` load balancer service and the DocumentDB test-default password `Admin100`.

```sh
python3 scripts/test-scripts/mongo-python-data-pusher.py
```
