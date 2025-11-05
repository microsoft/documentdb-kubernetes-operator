# DocumentDB Kubernetes Operator

A Kubernetes operator for managing DocumentDB clusters in your Kubernetes environment. This operator provides a native Kubernetes way to deploy, manage, and scale DocumentDB instances with MongoDB-compatible API.

## 🚀 What is DocumentDB Kubernetes Operator?

The DocumentDB Kubernetes Operator extends Kubernetes with Custom Resource Definitions (CRDs) to manage DocumentDB clusters declaratively. It leverages the power of Kubernetes controllers to ensure your DocumentDB deployments are always in the desired state.

### Key Features

- **Declarative Management**: Define your DocumentDB clusters using Kubernetes manifests
- **Automated Operations**: Automatic deployment, scaling, and lifecycle management
- **MongoDB Compatibility**: MongoDB API–compatible for seamless integration
- **Cloud Native**: Built on CloudNative-PG for robust PostgreSQL foundation
- **Helm Chart Support**: Easy installation and configuration via Helm
- **Enterprise Grade**: Multi-cloud support and high availability

## Quick Start


## 📚 Documentation

For installation guides, configuration options, and examples, visit our [documentation](https://documentdb.github.io/documentdb-kubernetes-operator).

## 🚀 Quick Start

Ready to get started? Check out our [Quick Start Guide](https://documentdb.github.io/documentdb-kubernetes-operator#quickstart) for step-by-step instructions to deploy your first DocumentDB cluster in minutes.

## Development Setup

```bash
# Clone the repository
git clone https://github.com/documentdb/documentdb-kubernetes-operator.git
cd documentdb-kubernetes-operator

# Build the operator (from the operator/src directory)
cd operator/src
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

- Create an [issue](https://github.com/documentdb/documentdb-kubernetes-operator/issues) for bug reports and feature requests
- Check our [documentation](https://documentdb.github.io/documentdb-kubernetes-operator) for common questions