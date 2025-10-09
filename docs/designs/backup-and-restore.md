# Backup and Restore Design

## Backup

### Backup CRD

We have our own Backup CRD and backup controller in the DocumentDB operator. When a Backup resource is created, it triggers a [Kubernetes Volume Snapshot](https://kubernetes.io/blog/2020/12/10/kubernetes-1.20-volume-snapshot-moves-to-ga/#what-is-a-volume-snapshot) on the primary instance of a DocumentDB cluster.

Since DocumentDB uses a [CloudNativePG (CNPG)](https://cloudnative-pg.io/) cluster as the backend, we leverage CNPG's backup functionality. When users create a DocumentDB Backup resource, the operator automatically creates a corresponding [CNPG Backup](https://cloudnative-pg.io/documentation/current/backup/) resource.

**Why not use CNPG Backup directly?**

In this phase, our Backup resource acts as a wrapper around CNPG Backup. We maintain our own CRD to support future enhancements:
- **Next phase:** Multi-region backup support
- **Future:** Multi-node backup capabilities

### Creating On-Demand Backups

Create an on-demand backup by applying the following resource:

```yaml
apiVersion: db.microsoft.com/preview
kind: Backup
metadata:
  name: backup-example
  namespace: documentdb-preview-ns
spec:
  cluster:
    name: documentdb-preview
```

## Scheduled Backup

### ScheduledBackup CRD

The ScheduledBackup CRD enables automated, recurring backups using [cron expressions](https://en.wikipedia.org/wiki/Cron).

**Why not use CNPG ScheduledBackup?**

CNPG's [ScheduledBackup](https://cloudnative-pg.io/documentation/current/backup/#scheduled-backups) creates CNPG Backup resources directly. Since we have our own Backup CRD with custom logic, we need our own ScheduledBackup implementation.

### Creating Scheduled Backups

Create a scheduled backup using a cron expression:

```yaml
apiVersion: db.microsoft.com/preview
kind: ScheduledBackup
metadata:
  name: backup-example
  namespace: documentdb-preview-ns
spec:
  schedule: "0 0 0 * * *"  # Daily at midnight
  cluster:
    name: documentdb-preview
```

## Retention Policy

Retention policies control how long backups are preserved before automatic deletion. The DocumentDB operator supports retention policies at multiple levels:

### Cluster-Level Retention

**Field:** `cluster.spec.backup.retentionPeriod`

**Purpose:** Defines how long backups are retained after the parent cluster is deleted.

**Example:**
```yaml
apiVersion: db.microsoft.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview
spec:
  backup:
    retentionPeriod: "30d"  # Retain backups for 30 days after cluster deletion
  # ...other fields...
```

### Backup-Level Retention

**Field:** `backup.spec.retentionPeriod`

**Purpose:** Defines how long an individual on-demand backup is retained before automatic deletion.

**Example:**
```yaml
apiVersion: db.microsoft.com/preview
kind: Backup
metadata:
  name: backup-example
spec:
  cluster:
    name: documentdb-preview
  retentionPeriod: "7d"  # Automatically delete after 7 days
```

### ScheduledBackup Retention

**Field:** `scheduledBackup.spec.retentionPeriod`

**Purpose:** Defines how long backups created by the scheduled job are retained.

**Example:**
```yaml
apiVersion: db.microsoft.com/preview
kind: ScheduledBackup
metadata:
  name: backup-example
spec:
  schedule: "0 0 0 * * *"
  cluster:
    name: documentdb-preview
  retentionPeriod: "14d"  # Keep scheduled backups for 14 days
```

Note: CNPG does not yet support retention policies for volume snapshots. This is an ongoing discussion in the CNPG community (see [issue #6009](https://github.com/cloudnative-pg/cloudnative-pg/issues/6009)).


## Deletion Behavior

- **Deleting a Backup resource:** Immediately deletes the associated volume snapshot
- **Deleting a ScheduledBackup resource:** Stops creating new backups but does not delete existing backups created by that schedule
- **Deleting a Cluster:** Backups are retained according to the cluster's `retentionPeriod` setting

## Restore

### Recovery from Backup

The operator supports bootstrapping a new cluster from an existing backup. In-place restoration is not currently supported.

**Recovery Example:**

```yaml
apiVersion: db.microsoft.com/preview
kind: DocumentDB
metadata:
  name: documentdb-preview-restore
  namespace: documentdb-preview-ns
spec:
  nodeCount: 1
  instancesPerNode: 1
  documentDBImage: ghcr.io/microsoft/documentdb/documentdb-local:16
  resource:
    pvcSize: 10Gi
  exposeViaService:
    serviceType: ClusterIP
  bootstrap:
    recovery:
      backup:
        name: backup-example
```

## Prerequisites

### VolumeSnapshotClass

Before taking volume snapshots, users must create a [VolumeSnapshotClass](https://kubernetes.io/docs/concepts/storage/volume-snapshot-classes/). The driver specified in the VolumeSnapshotClass depends on the underlying storage class.

**Example for Azure:**

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: azure-disk-snapclass
  annotations:
    snapshot.storage.kubernetes.io/is-default-class: "true"
driver: disk.csi.azure.com
deletionPolicy: Delete
```
### Open Questions

- **Should the operator automatically create a VolumeSnapshotClass if one doesn't exist?**
  - Current approach: Users must manually create a VolumeSnapshotClass before creating backups
  - Concern: This may not be user-friendly - should the operator automatically create a default VolumeSnapshotClass when a backup is requested if none exists?
  - Considerations:
    - Different cloud providers require different CSI drivers (e.g., `disk.csi.azure.com` for Azure, `ebs.csi.aws.com` for AWS)
    - The operator would need cloud provider detection logic
    - Users might have specific preferences for snapshot configurations

