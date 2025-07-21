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
helm install documentdb-operator oci://ghcr.io/microsoft/documentdb-kubernetes-operator/documentdb-operator --version 0.0.1 --namespace documentdb-operator --create-namespace
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
documentdbs.db.microsoft.com
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
# Connect using mongosh:
# mongosh 127.0.0.1:10260 -u k8s_secret_user -p K8sSecret100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
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

> **Note:** The sidecar injector plugin requires the secret to be named `documentdb-credentials` and must contain `username` and `password` keys. The plugin will automatically inject these as `USERNAME` and `PASSWORD` environment variables into the DocumentDB gateway container.


### Deploy a DocumentDB cluster

Create a single-node DocumentDB cluster:

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
NAME                 AGE
documentdb-preview   28m
```

### Connect to the DocumentDB cluster

The DocumentDB `Pod` has the Gateway container running as a sidecar. To keep things simple, the quickstart does not use a public load balancer. So you can connect to the DocumentDB instance directly through the Gateway port `10260`. For both `minikube` and `kind`, this can be easily done using port forwarding:

```sh
kubectl port-forward pod/documentdb-preview-1 10260:10260 -n documentdb-preview-ns
```

Connect using [mongosh](https://www.mongodb.com/docs/mongodb-shell/install/):

```sh
mongosh 127.0.0.1:10260 -u default_user -p Admin100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
```

Execute the following commands to create a database and a collection, and insert some documents:

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

### Other options: Try the sample Python app and `LoadBalancer` service

#### Connect to DocumentDB using a Python app

In addition to `mongosh`, you can also use the sample Python program (that uses the PyMongo client) in the GitHub repository to execute operations on the DocumentDB instance. It inserts a sample document to a `movies` collection inside the `sample_mflix` database.

```sh
git clone https://github.com/microsoft/documentdb-kubernetes-operator
cd documentdb-kubernetes-operator/scripts/test-scripts

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

#### Use a `LoadBalancer` service

For the quickstart, you connected to DocumentDB using port forwarding. If you are using a Kubernetes cluster in the cloud (for example, [Azure Kubernetes Service](https://learn.microsoft.com/en-us/azure/aks/)), and want to use a `LoadBalancer` service instead, enable it in the `DocumentDB` spec as follows:

```yaml
exposeViaService:
    serviceType: LoadBalancer
```

> `LoadBalancer` service is also supported in [minikube](https://minikube.sigs.k8s.io/docs/handbook/accessing/) and [kind](https://kind.sigs.k8s.io/docs/user/loadbalancer).

List the `Service`s and verify:

```sh
kubectl get services -n documentdb-preview-ns
```

This will create a `LoadBalancer` service named `documentdb-service-documentdb-preview` for the DocumentDB cluster. You can then access the DocumentDB instance using the external IP of the service.

```text
NAME                                    TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)           AGE
documentdb-preview-r                    ClusterIP      10.0.216.38    <none>          5432/TCP          26m
documentdb-preview-ro                   ClusterIP      10.0.31.103    <none>          5432/TCP          26m
documentdb-preview-rw                   ClusterIP      10.0.118.26    <none>          5432/TCP          26m
documentdb-service-documentdb-preview   LoadBalancer   10.0.228.243   52.149.56.216   10260:30312/TCP   27m
```

> If you are using the Python program to connect to DocumentDB, make sure to update the script's `host` variable with the external IP of your `documentdb-service-documentdb-preview` LoadBalancer service. Additionally, ensure that you update the default `password` in the script or, preferably, use environment variables to securely manage sensitive information like passwords.

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
  documentdbs.db.microsoft.com
```