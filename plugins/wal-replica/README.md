# WAL Receiver Pod Manager (CNPG-I Plugin)

This plugin adds an optional standalone WAL receiver (pg_receivewal) Pod/Deployment
alongside a [CloudNativePG](https://github.com/cloudnative-pg/cloudnative-pg/) Cluster.
It reconciles a Deployment named
`<cluster-name>-wal-receiver` that continuously streams WAL files from the primary
cluster using `pg_receivewal`, supporting synchronous mode.

## Parameters

Add the plugin in the Cluster spec (example):

```yaml
spec:
	plugins:
		- name: cnpg-i-wal-replica.documentdb.io
			parameters:
				image: "ghcr.io/cloudnative-pg/postgresql:16"
				replicationHost: cluster-name-rw
				synchronous: "enabled"            # optional (default true)
				walDirectory: /var/lib/wal     # optional (default /var/lib/wal)
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| image | string | ghcr.io/cloudnative-pg/postgresql:16 | Image providing pg_receivewal |
| replicationHost | string | <cluster>-rw | Host to connect for streaming |
| synchronous | bool | true | Add --synchronous flag to pg_receivewal |
| walDirectory | string | /var/lib/wal | Local directory to store WAL |

The Deployment exposes a metrics port (9187) and creates a Service with the same name.

## Build

```bash
go build -o bin/cnpg-i-wal-replica main.go
```

## Status

The plugin status reflects only whether it is enabled.

## Future Work

* Add PVC / volume configuration for WAL directory
* Expose resource requests/limits and security context
* Garbage collection / retention policy for archived WAL
* Liveness/readiness refinements

