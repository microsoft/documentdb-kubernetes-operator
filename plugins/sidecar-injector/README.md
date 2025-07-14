# CNPG-I Sidecar-Injector Plugin

A [CNPG-I](https://github.com/cloudnative-pg/cnpg-i) plugin to add
Document DB gateway sidecar to the pods of
[CloudNativePG](https://github.com/cloudnative-pg/cloudnative-pg/) clusters.

This plugin uses
the [pluginhelper](https://github.com/cloudnative-pg/cnpg-i-machinery/tree/main/pkg/pluginhelper)
from [`cnpg-i-machinery`](https://github.com/cloudnative-pg/cnpg-i-machinery) following the CNPG-I hello-world plugin.

## Running the plugin

**To Build:** `go build -o bin/cnpg-i-sidecar-injector main.go`

To see the plugin in execution, you need to have a Kubernetes cluster running
(we'll use [Kind](https://kind.sigs.k8s.io)) and the
[CloudNativePG](https://github.com/cloudnative-pg/cloudnative-pg/) operator
installed. The plugin also requires certificates to communicate with the
operator, hence we are also installing [cert-manager](https://cert-manager.io/)
to manage them.

