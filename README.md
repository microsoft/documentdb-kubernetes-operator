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


## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on how to get started.

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
- Review existing [discussions](https://github.com/microsoft/documentdb-kubernetes-operator/discussions) in the community

## 🏷️ Project Status

This project is currently in **preview** and actively developed. APIs may change as we work toward a stable release.

---

**Made with ❤️ by Microsoft**