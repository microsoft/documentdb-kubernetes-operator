# DocumentDB Kubernetes Operator

A Kubernetes operator for managing DocumentDB clusters in your Kubernetes environment. This operator provides a native Kubernetes way to deploy, manage, and scale DocumentDB instances with MongoDB-compatible API.

## 🚀 What is DocumentDB Kubernetes Operator?

The DocumentDB Kubernetes Operator extends Kubernetes with Custom Resource Definitions (CRDs) to manage DocumentDB clusters declaratively. It leverages the power of Kubernetes controllers to ensure your DocumentDB deployments are always in the desired state.

### Key Features

- **Declarative Management**: Define your DocumentDB clusters using Kubernetes manifests
- **Automated Operations**: Automatic deployment, scaling, and lifecycle management
- **MongoDB Compatibility**: Full MongoDB API compatibility for seamless application integration
- **Cloud Native**: Built on CloudNative-PG for robust PostgreSQL foundation
- **Helm Chart Support**: Easy installation and configuration via Helm
- **Production Ready**: Designed for enterprise-grade deployments

## ⚡ Quick Start

To get started with the DocumentDB Kubernetes Operator, follow our comprehensive [Quick Start Guide](https://microsoft.github.io/documentdb-kubernetes-operator/v1/)

## 📚 Documentation

For comprehensive documentation, installation guides, configuration options, and examples, visit our [GitHub Pages documentation](https://microsoft.github.io/documentdb-kubernetes-operator).

### Quick Links

- [Installation Guide](https://microsoft.github.io/documentdb-kubernetes-operator/v1/quick-start)

```sh
kubectl port-forward pod/documentdb-preview-1 10260:10260 -n documentdb-preview-ns
```

Connect using [mongosh](https://www.mongodb.com/docs/mongodb-shell/install/). Use the username and password from the `documentdb-credentials` secret you created earlier:

```sh
mongosh 127.0.0.1:10260 -u k8s_secret_user -p K8sSecret100 --authenticationMechanism SCRAM-SHA-256 --tls --tlsAllowInvalidCertificates
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

In addition to `mongosh`, you can also use the sample Python program (that uses the PyMongo client) in the GitHub repository to execute operations on the DocumentDB instance. It inserts a sample document to a `clubs` collection inside the `soccer_league` database.

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
 'name': 'Manchester United',
 'country': 'England',
 'founded': 1878,
 'stadium': 'Old Trafford',
 'league': 'Premier League',
 'titles': ['Premier League', 'FA Cup', 'Champions League']}
```

You can verify this using the `mongosh` shell:

```sh
use soccer_league
db.clubs.find()
```

Output:

```text
[direct: mongos] testdb> use soccer_league
switched to db soccer_league
[direct: mongos] soccer_league> 

[direct: mongos] soccer_league> db.clubs.find()
[
  {
    _id: ObjectId('682c54f9505b85fba77ed154'),
    name: 'Manchester United',
    country: 'England',
    founded: 1878,
    stadium: 'Old Trafford',
    league: 'Premier League',
    titles: [ 'Premier League', 'FA Cup', 'Champions League' ]
  }
]
```

#### Use a `LoadBalancer` service

For the quickstart, you connected to DocumentDB using port forwarding. If you are using a Kubernetes cluster in the cloud (for example, [Azure Kubernetes Service](https://learn.microsoft.com/en-us/azure/aks/)), and want to use a `LoadBalancer` service instead, enable it in the `DocumentDB` spec as follows:

```yaml
exposeViaService:
    serviceType: ClusterIP
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

### Development Setup

```bash
# Clone the repository
git clone https://github.com/microsoft/documentdb-kubernetes-operator.git
cd documentdb-kubernetes-operator

# Build the operator
make build

# Run tests
make test

# Deploy to your cluster
make deploy
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔒 Security

For security concerns, please review our [Security Policy](SECURITY.md).

## 💬 Support

- Create an [issue](https://github.com/microsoft/documentdb-kubernetes-operator/issues) for bug reports and feature requests
- Check our [documentation](https://microsoft.github.io/documentdb-kubernetes-operator) for common questions