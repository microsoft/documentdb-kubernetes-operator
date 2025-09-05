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
				enabled: "true"
				image: "ghcr.io/cloudnative-pg/postgresql:16"
				replicationUser: streaming_replica
				replicationPasswordSecretName: cluster-replication
				replicationPasswordSecretKey: password # optional (default: password)
				synchronous: "true"            # optional (default true)
				walDirectory: /var/lib/wal     # optional (default /var/lib/wal)
				# replicationHost: override-host.example # optional
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| enabled | bool | false | Enable or disable the plugin |
| image | string | ghcr.io/cloudnative-pg/postgresql:16 | Image providing pg_receivewal |
| replicationHost | string | <cluster>-rw | Host to connect for streaming |
| replicationUser | string | streaming_replica | Replication user |
| replicationPasswordSecretName | string | (required when enabled) | Secret containing replication password |
| replicationPasswordSecretKey | string | password | Key in the secret |
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

