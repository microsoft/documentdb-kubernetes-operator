# DocumentDB Kubernetes Operator

The DocumentDB Kubernetes Operator is an open-source project to run and manage [DocumentDB](https://github.com/microsoft/documentdb) on Kubernetes. `DocumentDB` is the engine powering vCore-based Azure Cosmos DB for MongoDB. It is built on top of PostgreSQL and offers a native implementation of document-oriented NoSQL database, enabling CRUD operations on BSON data types.

As part of a DocumentDB cluster installation, the operator deploys and manages a set of PostgreSQL instance(s), the [DocumentDB Gateway](https://github.com/microsoft/documentdb/tree/main/pg_documentdb_gw), as well as other Kubernetes resources. While PostgreSQL is used as the underlying storage engine, the gateway ensures that you can connect to the DocumentDB cluster using MongoDB-compatible drivers, APIs, and tools.

> **Note:** This project is under active development but not yet recommended for production use. We welcome your feedback and contributions!

## Quickstart

This quickstart guide will walk you through the steps to install the operator, deploy a DocumentDB cluster, access it using `mongosh`, and perform basic operations.

### Prerequisites

- [Helm](https://helm.sh/docs/intro/install/) installed.
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/) installed.
- A local Kubernetes cluster such as [minikube](https://minikube.sigs.k8s.io/docs/start/), or [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed. You are free to use any other Kubernetes cluster, but that's not a requirement for this quickstart.
- Install [mongosh](https://www.mongodb.com/docs/mongodb-shell/install/) to connect to the DocumentDB cluster.

### Start a local Kubernetes cluster

If you are using `minikube`, use the following command:

```sh
minikube start
```

If you are using `kind`, use the following command:

```sh
kind create cluster
```

### Install `cert-manager`

[cert-manager](https://cert-manager.io/docs/) is used to manage TLS certificates for the DocumentDB cluster.

> If you already have `cert-manager` installed, you can skip this step.

```sh
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true
```

Verify that `cert-manager` is installed correctly:

```sh
kubectl get pods -n cert-manager
```

Output:

```text
NAMESPACE           NAME                                            READY   STATUS    RESTARTS
cert-manager        cert-manager-6794b8d569-d7lwd                   1/1     Running   0
cert-manager        cert-manager-cainjector-7f69cd69f7-pd9bc        1/1     Running   0          
cert-manager        cert-manager-webhook-6cc5dccc4b-7jmrh           1/1     Running   0          
```

### Install `documentdb-operator` using the Helm chart

> The DocumentDB operator utilizes the [CloudNativePG operator](https://cloudnative-pg.io/docs/) behind the scenes, and installs it in the `cnpg-system` namespace. At this point, it is assumed that the CloudNativePG operator is **not** pre-installed in your cluster.

Use the following command to install the DocumentDB operator:

```sh
# Add the Helm repository
helm repo add documentdb https://documentdb.github.io/documentdb-kubernetes-operator
helm repo update

# Install the operator
helm install documentdb-operator documentdb/documentdb-operator \
  --namespace documentdb-operator \
  --create-namespace \
  --wait
```

This will install the operator in the `documentdb-operator` namespace. Verify that it is running:

```sh
kubectl get deployment -n documentdb-operator
```

Output:

```text
NAME                  READY   UP-TO-DATE   AVAILABLE   AGE
documentdb-operator   1/1     1            1           113s
```

You should also see the DocumentDB operator CRDs installed in the cluster:

```sh
kubectl get crd | grep documentdb
```

Output:

```text
documentdbs.db.documentdb.com
```

### Store DocumentDB credentials in K8s Secret

Before deploying the DocumentDB cluster, create a Kubernetes secret to store the DocumentDB credentials. The sidecar injector plugin will automatically inject these credentials as environment variables into the DocumentDB gateway container.

Create the secret with your desired username and password:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: documentdb-preview-ns
---
# DocumentDB Credentials Secret
# 
# Login credentials:
# Username: k8s_secret_user
# Password: K8sSecret100
#
# Connect using mongosh (port-forward):
# mongosh 127.0.0.1:10260 -u k8s_secret_user -p K8sSecret100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
#
# Connect using connection string (port-forward):
# mongosh "mongodb://k8s_secret_user:K8sSecret100@127.0.0.1:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0"
#

apiVersion: v1
kind: Secret
metadata:
  name: documentdb-credentials
  namespace: documentdb-preview-ns
type: Opaque
stringData:
  username: k8s_secret_user     
  password: K8sSecret100        
EOF
```

Verify the secret is created:

```sh
kubectl get secret documentdb-credentials -n documentdb-preview-ns
```

Output:

```text
NAME                     TYPE     DATA   AGE
documentdb-credentials   Opaque   2      10s
```

> **Note:** By default the operator expects a credentials secret named `documentdb-credentials` containing `username` and `password` keys. You can override the secret name by setting `spec.documentDbCredentialSecret` in your `DocumentDB` resource. Whatever name you configure (or the default) will be used by the sidecar injector to project the values as `USERNAME` and `PASSWORD` environment variables into the gateway sidecar container.


### Deploy a DocumentDB cluster

Create a single-node DocumentDB cluster:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: documentdb-preview-ns
---
apiVersion: db.documentdb.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDbCredentialSecret: documentdb-credentials
  resource:
    storage:
      pvcSize: 10Gi
  exposeViaService:
    serviceType: ClusterIP
EOF
```

Wait for the DocumentDB cluster to be fully initialized. Verify that it is running:

```sh
kubectl get pods -n documentdb-preview-ns
```

Output:

```text
NAME                   READY   STATUS    RESTARTS   AGE
documentdb-preview-1   2/2     Running   0          26m
```

You can also check the DocumentDB CRD instance:

```sh
kubectl get DocumentDB -n documentdb-preview-ns
```

Output:

```text
NAME                 STATUS                     CONNECTION STRING
documentdb-preview   Cluster in healthy state   mongodb://$(kubectl get secret documentdb-credentials -n documentdb-preview-ns -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret documentdb-credentials -n documentdb-preview-ns -o jsonpath='{.data.password}' | base64 -d)@10.0.29.01:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0
```

### Connect to the DocumentDB cluster

Once you have deployed your DocumentDB cluster, you can connect using different methods depending on your service type. Choose the approach that best fits your deployment strategy:

#### Option 1: ClusterIP Service (Default - for local development)

The default deployment uses `ClusterIP` service type. To connect from your local machine, use port forwarding:

**Step 1:** Set up port forwarding (keep this terminal open):
```sh
kubectl port-forward pod/documentdb-preview-1 10260:10260 -n documentdb-preview-ns
```

**Step 2:** In a **new terminal**, connect using [mongosh](https://www.mongodb.com/docs/mongodb-shell/install/):

```sh
# Traditional format (via port-forward)
mongosh 127.0.0.1:10260 -u k8s_secret_user -p K8sSecret100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates

# Or connection string format (via port-forward)
mongosh "mongodb://k8s_secret_user:K8sSecret100@127.0.0.1:10260/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0"
```

#### Option 2: LoadBalancer Service (For cloud deployments)

If you prefer direct external access (recommended for cloud environments like Azure AKS), deploy with `LoadBalancer` service type:

**Step 1:** Deploy DocumentDB with LoadBalancer service:
```sh
cat <<EOF | kubectl apply -f -
apiVersion: db.documentdb.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview
  namespace: documentdb-preview-ns
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDbCredentialSecret: documentdb-credentials
  resource:
    storage:
      pvcSize: 10Gi
  exposeViaService:
    serviceType: LoadBalancer
EOF
```

**Step 2:** Wait for the external IP to be assigned:
```sh
kubectl get services -n documentdb-preview-ns -w
```

You should see something like:
```text
NAME                                    TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)           AGE
documentdb-service-documentdb-preview   LoadBalancer   10.0.228.243   52.149.56.216   10260:30312/TCP   2m
```

**Step 3:** Connect directly using the external IP:
```sh
# Get the connection string with external IP automatically populated
kubectl get documentdb documentdb-preview -n documentdb-preview-ns -o jsonpath='{.status.connectionString}'

# Copy the output and use it directly with mongosh
mongosh "PASTE_CONNECTION_STRING_HERE"
```

> **Note:** `LoadBalancer` service is supported in cloud environments (Azure AKS, AWS EKS, GCP GKE), as well as local development with [minikube](https://minikube.sigs.k8s.io/docs/handbook/accessing/) and [kind](https://kind.sigs.k8s.io/docs/user/loadbalancer).

### Work with Data

Once connected, execute the following commands to create a database and a collection, and insert some documents:

```sh
use testdb

db.createCollection("test_collection")

db.test_collection.insertMany([
  { name: "Alice", age: 30 },
  { name: "Bob", age: 25 },
  { name: "Charlie", age: 35 }
])

db.test_collection.find()
```

Output:

```text
[direct: mongos] test> use testdb
switched to db testdb
[direct: mongos] testdb> db.createCollection("test_collection")
{ ok: 1 }
[direct: mongos] testdb> db.test_collection.insertMany([
...   { name: "Alice", age: 30 },
...   { name: "Bob", age: 25 },
...   { name: "Charlie", age: 35 }
... ])
{
  acknowledged: true,
  insertedIds: {
    '0': ObjectId('682c3b06491dc99ae02b3fed'),
    '1': ObjectId('682c3b06491dc99ae02b3fee'),
    '2': ObjectId('682c3b06491dc99ae02b3fef')
  }
}
[direct: mongos] testdb> db.test_collection.find()
[
  { _id: ObjectId('682c3b06491dc99ae02b3fed'), name: 'Alice', age: 30 },
  { _id: ObjectId('682c3b06491dc99ae02b3fee'), name: 'Bob', age: 25 },
  {
    _id: ObjectId('682c3b06491dc99ae02b3fef'),
    name: 'Charlie',
    age: 35
  }
]
```

### Other options: Try the sample Python app

#### Connect to DocumentDB using a Python app

In addition to `mongosh`, you can also use the sample Python program (that uses the PyMongo client) in the GitHub repository to execute operations on the DocumentDB instance. It inserts a sample document to a `movies` collection inside the `sample_mflix` database.

```sh
git clone https://github.com/documentdb/documentdb-kubernetes-operator
cd documentdb-kubernetes-operator/operator/src/scripts/test-scripts

pip3 install pymongo

python3 mongo-python-data-pusher.py
```

Output:

```text
Inserted document ID: 682c54f9505b85fba77ed154
{'_id': ObjectId('682c54f9505b85fba77ed154'),
 'cast': ['Olivia Colman', 'Emma Stone', 'Rachel Weisz'],
 'directors': ['Yorgos Lanthimos'],
 'genres': ['Drama', 'History'],
 'rated': 'R',
 'runtime': 121,
 'title': 'The Favourite MongoDB Movie',
 'type': 'movie',
 'year': 2018}
```

You can verify this using the `mongosh` shell:

```sh
use sample_mflix
db.movies.find()
```

Output:

```text
[direct: mongos] testdb> use sample_mflix
switched to db sample_mflix
[direct: mongos] sample_mflix> 

[direct: mongos] sample_mflix> db.movies.find()
[
  {
    _id: ObjectId('682c54f9505b85fba77ed154'),
    title: 'The Favourite MongoDB Movie',
    genres: [ 'Drama', 'History' ],
    runtime: 121,
    rated: 'R',
    year: 2018,
    directors: [ 'Yorgos Lanthimos' ],
    cast: [ 'Olivia Colman', 'Emma Stone', 'Rachel Weisz' ],
    type: 'movie'
  }
]
```

> If you are using the Python program to connect to DocumentDB, make sure to update the script's `host` variable with the appropriate IP address based on your service type (127.0.0.1 for ClusterIP with port-forward, or the external IP for LoadBalancer service). Additionally, ensure that you update the default `password` in the script or, preferably, use environment variables to securely manage sensitive information like passwords.

## Configuration and Advanced Topics

Now that you have a basic DocumentDB cluster running, you may want to explore advanced configuration options and operational guides:

### Sidecar Injector Plugin Configuration

The DocumentDB operator uses a sidecar injector plugin to automatically inject the DocumentDB Gateway container into PostgreSQL pods. This plugin supports multiple configuration parameters including:

- **Gateway Image Configuration**: Customize which DocumentDB Gateway container image is used
- **Pod Labels and Annotations**: Add custom metadata to injected pods

For detailed information on configuring the sidecar injector plugin, see: [Sidecar Injector Plugin Configuration](../../developer-guides/sidecar-injector-plugin-configuration.md)


### Multi-Cloud Deployment

The DocumentDB operator supports deployment across multiple cloud environments and Kubernetes distributions. For guidance on multi-cloud deployments, see: [Multi-Cloud Deployment Guide](../../../documentdb-playground/multi-clould-setup/multi-cloud-deployment-guide.md)

### TLS Setup

For advanced TLS configuration and testing:

- [TLS Setup Guide](../../../documentdb-playground/tls/README.md) - Complete TLS configuration guide
- [E2E Testing](../../../documentdb-playground/tls/E2E-TESTING.md) - Comprehensive testing procedures


## Clean Up
### Delete the DocumentDB cluster and other resources

```sh
kubectl delete DocumentDB documentdb-preview -n documentdb-preview-ns
```

The `Pod` should now be terminated:

```sh
kubectl get pods -n documentdb-preview-ns
```

Uninstall the DocumentDB operator:

```sh
helm uninstall documentdb-operator --namespace documentdb-operator
```

Output:

```text
These resources were kept due to the resource policy:
[CustomResourceDefinition] poolers.postgresql.cnpg.io
[CustomResourceDefinition] publications.postgresql.cnpg.io
[CustomResourceDefinition] scheduledbackups.postgresql.cnpg.io
[CustomResourceDefinition] subscriptions.postgresql.cnpg.io
[CustomResourceDefinition] backups.postgresql.cnpg.io
[CustomResourceDefinition] clusterimagecatalogs.postgresql.cnpg.io
[CustomResourceDefinition] clusters.postgresql.cnpg.io
[CustomResourceDefinition] databases.postgresql.cnpg.io
[CustomResourceDefinition] imagecatalogs.postgresql.cnpg.io

release "documentdb-operator" uninstalled
```

Verify that the `Pod` is removed:

```sh
kubectl get pods -n documentdb-preview-ns
```

Delete namespace, and CRDs:

```sh
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
  documentdbs.db.documentdb.com
```