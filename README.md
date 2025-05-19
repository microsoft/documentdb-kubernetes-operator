# DocumentDB Kubernetes Operator

> **⚠️ WARNING: This setup is for demo and testing only. It is NOT recommended for production use.**

The DocumentDB Kubernetes Operator is an open-source project to run and manage [DocumentDB](https://github.com/microsoft/documentdb) on using Kubernetes. `DocumentDB` is the engine powering vCore-based Azure Cosmos DB for MongoDB. It is built on top of PostgreSQL and offers a native implementation of document-oriented NoSQL database, enabling CRUD operations on BSON data types.

# Quick Start

## Prerequisites

- Helm installed
- kubectl installed
- Azure CLI installed (if you are using Azure Kubernetes Service (AKS))
- Install `cert-manager`
  ```sh
  helm repo add jetstack https://charts.jetstack.io
  helm repo update
  helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true
  ```
  Make sure the following cert-manager pods are running.
  ```sh
  kubectl get pods -A
  NAMESPACE           NAME                                            READY   STATUS    RESTARTS   
  cert-manager        cert-manager-6794b8d569-d7lwd                   1/1     Running   0          
  cert-manager        cert-manager-cainjector-7f69cd69f7-pd9bc        1/1     Running   0          
  cert-manager        cert-manager-webhook-6cc5dccc4b-7jmrh           1/1     Running   0          
  ```

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

**Note:** The DocumentDB operator installs and utilizes the `cnpg-operator` under the hood in the `cnpg-system` namespace. At this point, the DocumentDB operator expects that the `cnpg-operator` is **NOT** pre-installed in your cluster.

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
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 10Gi
  publicLoadBalancer:
    enabled: false
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
```

You shouold see LoadBalancer service `documentdb-service-documentdb-preview` only if you enabled it in the spec with:

```yaml
publicLoadBalancer:
    enabled: ture
```

```sh
kubectl get pods -n documentdb-preview-ns
NAME                   READY   STATUS    RESTARTS   AGE
documentdb-preview-1   2/2     Running   0          26m
```

### 5. Test pushing some dummy documents into your DocumentDB

We have a test Python script that uses a PyMongo client to push a test document into DocumentDB and read it back. Update the script with the external IP of your `documentdb-service-documentdb-preview` load balancer service and the DocumentDB test-default password `Admin100`.

```sh
python3 scripts/test-scripts/mongo-python-data-pusher.py
```

**Note:** If you are not using a public load balancer, you can connect directly to your DocumentDB pod on Gateway port 10260. If you are using Minikube or Kind on your local machine, you need to forward the DocumentDB Gateway port first.

```sh
kubectl port-forward pod/documentdb-preview-1 10260:10260 -n documentdb-preview-ns
```

Then you can connect to the pod using mongosh:

```sh
mongosh 127.0.0.1:10260 -u default_user -p Admin100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
```

### 6. Clean Up

#### 6.1: Delete the Document DB workload.

```sh
cat <<EOF | kubectl delete -f -
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
  publicLoadBalancer:
    enabled: false
EOF
```

#### 6.2: Uninstall the DocumentDB Operator, namespace, and CRDs.

```sh
helm uninstall documentdb-operator --namespace documentdb-operator
kubectl delete namespace documentdb-operator
kubectl delete crd backups.postgresql.cnpg.io \
  clusterimagecatalogs.postgresql.cnpg.io \
  clusters.postgresql.cnpg.io \
  databases.postgresql.cnpg.io \
  imagecatalogs.postgresql.cnpg.io \
  poolers.postgresql.cnpg.io \
  publications.postgresql.cnpg.io \
  scheduledbackups.postgresql.cnpg.io \
  subscriptions.postgresql.cnpg.io \
  documentdbs.db.microsoft.com
```
