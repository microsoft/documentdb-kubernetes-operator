# Plugin Development

This section of the documentation illustrates the CNPG-I capabilities used by
the sidecar-injector plugin, how the plugin implementation uses them, and how
developers can build and deploy the plugin.

## Concepts

### Identity

The Identity interface defines the features supported by the plugin and is the
only interface that must always be implemented.

This information is essential for the operator to discover the plugin's
capabilities during startup.

The Identity interface provides:

- A mechanism for plugins to report readiness probes. Readiness is a
  prerequisite for receiving events, and plugins are expected to always report
  the most accurate readiness data available.
- The capabilities reported by the plugin, which determine the subsequent calls
  the plugin will receive.
- Metadata about the plugin.

[API reference](https://github.com/cloudnative-pg/cnpg-i/blob/main/proto/identity.proto)

### Capabilities

This plugin implements the Operator and the Lifecycle capabilities.

#### Operator

This feature enables the plugin to receive events about the cluster creation and
mutations, this is defined by the following

``` proto
// ValidateCreate improves the behavior of the validating webhook that
// is called on creation of the Cluster resources
rpc ValidateClusterCreate(OperatorValidateClusterCreateRequest) returns (OperatorValidateClusterCreateResult) {}

// ValidateClusterChange improves the behavior of the validating webhook of
// is called on updates of the Cluster resources
rpc ValidateClusterChange(OperatorValidateClusterChangeRequest) returns (OperatorValidateClusterChangeResult) {}

// MutateCluster fills in the defaults inside a Cluster resource
rpc MutateCluster(OperatorMutateClusterRequest) returns (OperatorMutateClusterResult) {}
```

This interface allows plugins to implement important features like:

1. validating the cluster manifest during the creation and mutations
   (it is expected that the plugin validate the parameters assigned to their
   configuration).

2. mutating the cluster object before it is submitted to kubernetes API server,
   for example to set default values for the plugin parameters.

[API reference](https://github.com/cloudnative-pg/cnpg-i/blob/main/proto/operator.proto)

The sidecar-injector plugin is using this to validate used-defined parameters, and to
set default values for the labels and annotations applied by the plugin if not
specified by the user.

#### Lifecycle

This feature enables the plugin to receive events and create patches for
Kubernetes resources `before` they are submitted to the API server.

To use this feature, the plugin must specify the resource and operation it wants
to be notified of.

Some examples of what it can be achieved through the lifecycle:

- add volume, volume mounts, sidecar containers, labels, annotations to pods,
  especially necessary when implementing custom backup solutions
- modify any resource with some annotations or labels
- add/remove finalizers

[API reference](https://github.com/cloudnative-pg/cnpg-i/blob/main/proto/operator_lifecycle.proto):

The sidecar-injector plugin is using this to add labels, annotations and a sidecar
to the pods.

## Implementation

### Identity

1. Define a struct inside the `internal/identity` package that implements
   the `pluginhelper.IdentityServer` interface.

2. Implement the following methods:

    - `GetPluginMetadata`: return human-readable information about the plugin.
    - `GetPluginCapabilities`: specify the features supported by the plugin. In
      the sidecar-injector example, the
      `PluginCapability_Service_TYPE_LIFECYCLE_SERVICE` is defined in the
      corresponding Go [file](../internal/lifecycle/lifecycle.go).
    - `Probe`: indicate whether the plugin is ready to serve requests; this
      example is stateless, so it will always be ready.

### Lifecycle

This example implements the lifecycle service capabilities to add labels and
annotations to the pods. The `OperatorLifecycleServer` interface is implemented
inside the `internal/lifecycle` package.

The `OperatorLifecycleServer` interface requires several methods:

- `GetCapabilities`: describe the resources and operations the plugin should be
  notified for

- `LifecycleHook`: is invoked for every operation against the Kubernetes API
  server that matches the specifications returned by `GetCapabilities`

  In this function, the plugin is expected to do pattern matching using
  the `Kind` and the operation `Type` and proceed with the proper logic.

### Operator

The operator interface offers a way for the plugin to interact with the Cluster
resource webhooks.

Do that, the plugin should implement
the [operator](https://github.com/cloudnative-pg/cnpg-i/blob/main/proto/operator.proto)
interface, specifically the `MutateCluster`, `ValidateClusterCreate`,
and `ValidateClusterChange` rpc calls.

- `MutateCluster`: enriches the plugin defaulting webhook

- `ValidateClusterCreate` and `ValidateClusterChange`: enriches the plugin
  validation logic.

The package `internal/operator` implements this interface.

### Startup Command

The plugin runs in its own pod, and its main command is implemented in
the `main.go` file.

This function uses the plugin helper library to create a GRPC server and manage
TLS.

Plugin developers are expected to use the `pluginhelper.CreateMainCmd`
to implement the `main` function, passing an implemented `Identity`
struct.

Further implementations can be registered within the callback function.

In the example we propose, that's done for **operator** and for the
**lifecycle** services in [file](../cmd/plugin/plugin.go):

``` proto
operator.RegisterOperatorServer(server, operatorImpl.Implementation{})
lifecycle.RegisterOperatorLifecycleServer(server, lifecycleImpl.Implementation{})
```

## Build and deploy the plugin

Users can test their own changes to the plugin by building a container image
running it inside a Kubernetes cluster with CloudNativePG and cert-manager
installed.

### Local build

The repository provides a [`Taskfile`](https://taskfile.dev/) that contains
several helpful commands to test the plugin in
a [CNPG development environment](https://github.com/cloudnative-pg/cloudnative-pg/tree/main/contribute/e2e_testing_environment#the-local-kubernetes-cluster-for-testing).

By executing `task local-kind-deploy`, a container image containing the
executable of the repository will be built and loaded inside the kind cluster.

Having done that, the sidecar-injector plugin deployment will be applied.

### CI/CD build

The repository provides a GitHub Actions workflow that, on pushes, builds a
container image and generates a manifest file that can be used to deploy the
plugin. The manifest is attached to the workflow run as an artifact, and can be
applied to the cluster.
