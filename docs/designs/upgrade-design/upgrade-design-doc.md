# DocumentDB Kubernetes Operator Upgrade Design

## Overview

This document outlines the upgrade strategy for the DocumentDB Kubernetes operator, which provides a MongoDB-compatible API layer over PostgreSQL using the CloudNative-PG (CNPG) operator. The system consists of multiple components that require coordinated upgrades to ensure service continuity and data integrity.

## Required Knowledge

The following sections provide essential background information for implementing DocumentDB operator upgrades:

### 1. Kubernetes Operators
Kubernetes operators extend the API to manage complex applications through custom resources and controllers that continuously reconcile desired state. Operators use admission webhooks to validate and modify resources during creation and updates.

**Learn more**: [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) | [Operator White Paper](https://github.com/cncf/tag-app-delivery/blob/163962c4b1cd70d085107fc579e3e04c2e14d59c/operator-wg/whitepaper/Operator-WhitePaper_v1-0.md) | [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) | [Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)

### 2. DocumentDB Operator Architecture
The DocumentDB operator provides a MongoDB-compatible API over PostgreSQL by orchestrating multiple components: the operator controller, gateway containers for protocol translation, PostgreSQL clusters with DocumentDB extensions, and sidecar injection for seamless integration with CloudNative-PG (CNPG) operator.

**Learn more**: [DocumentDB Operator README](../../../README.md) | [CloudNative-PG Documentation](https://cloudnative-pg.io/documentation/)

### 3. Helm Chart Management
Helm makes Kubernetes application packaging and deployment easy by bundling multiple related resources into a single chart. We provide a DocumentDB operator Helm chart that customers can install with a single command, automatically deploying all necessary components including the DocumentDB operator, CNPG operator, sidecar injector, and associated configurations.

**Learn more**: [Helm Documentation](https://helm.sh/docs/) | [Managing CRDs with Helm](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) | [Helm Upgrade Process](https://helm.sh/docs/helm/helm_upgrade/)

### 4. PostgreSQL and Extensions
PostgreSQL version upgrades involve considerations for both the database engine and extensions, with major version upgrades requiring data migration and careful compatibility validation. Extension upgrades may modify schemas or data structures independently of the PostgreSQL version.

**Learn more**: [PostgreSQL Versioning Policy](https://www.postgresql.org/support/versioning/) | [PostgreSQL Upgrade Methods](https://www.postgresql.org/docs/current/upgrading.html) | [Extension Management](https://www.postgresql.org/docs/current/extend-extensions.html)

## Architecture Components

The DocumentDB operator consists of five main components distributed across different k8s namespaces and nodes:

![DocumentDB Kubernetes Architecture](documentdb-k8s-architecture.png)

### 1. DocumentDB Operator
A custom Kubernetes operator that manages DocumentDB custom resources and orchestrates the creation and lifecycle of CNPG PostgreSQL clusters. Runs in the `documentdb-operator` namespace on worker nodes.

### 2. Gateway Container  
A MongoDB protocol translator that runs as a sidecar container alongside PostgreSQL, converting MongoDB wire protocol requests into Postgres DocumentDB extension calls. Deployed in customer application namespaces on worker nodes.

### 3. PostgreSQL with DocumentDB Extension
A PostgreSQL server enhanced with DocumentDB extensions that enable MongoDB-like document storage and querying capabilities over a relational database. Deployed in customer application namespaces on worker nodes.

### 4. CNPG Sidecar Injector
An admission webhook that automatically injects the DocumentDb Gateway container into CNPG PostgreSQL pods during deployment. Runs in the `cnpg-system` namespace on worker nodes.

### 5. CNPG Operator
The CloudNative-PG operator that handles PostgreSQL cluster lifecycle management, including high availability, backups, and upgrades. Runs in the `cnpg-system` namespace on worker nodes.

### Component Communication Flow

**Control Plane Interaction:**
- All operators (DocumentDB, CNPG, Sidecar Injector) communicate with the Kubernetes API server running on control plane nodes
- The API server validates requests and stores resource definitions in etcd
- The API server directs kubelet agents on worker nodes to apply changes to pods and containers

**Data Plane Deployment:**
- DocumentDB clusters (PostgreSQL + Gateway containers) are deployed in customer-specified application namespaces
- kubelet on worker nodes manages the actual pod lifecycle and container execution
- Application traffic flows directly to Gateway containers in the application namespaces

## Design Principles

Our upgrade strategy follows four core principles:

### 1. Zero-Downtime Principle
All upgrades maintain service availability through rolling updates and automatic rollback on failure.

### 2. Backward Compatibility Principle  
Support N-2 API versions with deprecation cycles (6-months) for gradual migration.

### 3. Fail-Safe Operation Principle
Failed upgrades automatically rollback using atomic Helm operations and change detection.

### 4. Team Autonomy Principle
Clear separation: Kubernetes admins upgrade infrastructure, Database admins coordinate cluster migrations, Database developers execute application-specific migrations.

## Goals and Non-Goals

### Goals
- **Zero-downtime upgrades**: All DocumentDB components upgrade without service interruption
- **Gradual migration capability**: Support API version migration over weeks/months timeline
- **Automated rollback**: Failed upgrades automatically revert to previous stable state
- **Team independence**: Platform and application teams operate on separate timelines
- **Operational simplicity**: Minimize complexity for development teams
- **Data integrity**: Guarantee no data loss during upgrade processes

### Non-Goals
- **Unlimited version history**: Only support N-2 API versions (latest 3 versions maximum)
- **Cross-cloud migration**: Upgrades within same Kubernetes cluster only
- **Automatic data migration**: Breaking schema changes require manual planning
- **Zero-configuration experience**: Some operational knowledge required


## Versioning Strategy

**Important**: DocumentDB uses a **unified versioning strategy** where all components are versioned together for simplicity and compatibility assurance.

### Unified Versioning Approach

**Single Version for All Components:**
- **DocumentDB Operator Version**: `v1.2.3` controls all component versions
- **Gateway Image**: Automatically aligned (e.g., `ghcr.io/microsoft/documentdb/gateway:v1.2.3`)
- **PostgreSQL + Extension**: Automatically aligned (e.g., `mcr.microsoft.com/documentdb/documentdb:16.2-v1.2.3`)
- **Sidecar Injector**: Automatically aligned (e.g., `ghcr.io/microsoft/documentdb/sidecar-injector:v1.2.3`)
- **CNPG Operator**: Locked to DocumentDB version (e.g., DocumentDB v1.2.3 → CNPG v0.24.0)

**Benefits of Unified Versioning:**
- **Simplified Operations**: One version to track instead of managing multiple component versions
- **Guaranteed Compatibility**: All components tested together as a cohesive unit
- **Reduced Complexity**: Eliminates version matrix compatibility issues
- **Easier Rollbacks**: Single version rollback affects all components consistently
- **Clear Release Management**: Single release pipeline for all components

## Upgrade Strategies

DocumentDB uses a multi-version API approach where a single operator version supports multiple DocumentDB cluster versions simultaneously, enabling gradual migration without forcing upgrades.

### Three-Tier Responsibility Model

| Role | Primary Scope | Upgrade Ownership |
|------|---------------|------------------|
| Kubernetes Admin | Cluster infrastructure & operators | Phase 1 – Infrastructure upgrade |
| Database Admin (DBA) | Cluster fleet lifecycle & performance | Phase 2 – Cluster API migration |
| Application / Database Developer | App integration & validation | Phase 3 – Application validation |

### Multi-Version Support Architecture

- Operator v2: serves cluster API v1 + v2
- Operator v3: serves v1 (deprecated) + v2 + v3
- Operator v4: serves v2 + v3 + v4 (v1 removed)

Deprecation cadence: Version N introduces API vN; N+1 deprecates v(N-1); N+2 removes v(N-1). Operator maintains at most 2–3 active versions.

### Phase 1: Infrastructure Upgrade (Kubernetes Admin)
- Scope: Helm upgrade of unified operator chart (DocumentDB operator, Sidecar Injector, CNPG if required)
- Upgrades: controller, CRDs (add new fields keep old), webhooks, RBAC, sidecar injector, optional CNPG version
- Not Upgraded: existing cluster CRs, gateway image, postgres+extension images, application workloads
- Key Steps:
   - Helm dry-run validation
   - Atomic Helm upgrade (rollback on failure)
   - Operator & webhook health checks
   - Backward compatibility check (new operator reconciles existing clusters)
- Success Criteria:
   - All operator pods Ready
   - Existing clusters stay Healthy/Ready
   - New cluster with new API version can be created

### Phase 2: Cluster Migration (DBA)
Goal: Per-cluster apiVersion/spec bump (e.g. v1→v2) in controlled waves.
Steps per cluster:
1. Pre-checks (recent backup, replication healthy, capacity OK)
2. Server-side dry-run patch
3. Apply manifest (apiVersion + new spec fields)
4. Watch Ready condition & CNPG replication status
5. Smoke test: connect, CRUD, check gateway/extension versions
6. Record outcome or rollback (reapply previous manifest)
Rollback triggers: Ready timeout, sustained latency/error regression, high replication lag, crash loops.

### Phase 3: Application Validation (Developer)
Actions: Validate connectivity, CRUD/index/query latency, error rates, enable new feature flags if desired.
Escalate to DBA on regression; otherwise mark migration complete.

Benefits:
- Clear role separation
- Gradual adoption & controlled risk
- Side-by-side API versions until retirement
- Simple rollback boundaries

For detailed commands & YAML examples see [commands.md](./commands.md).

## Component-Specific Upgrade Considerations

While all components upgrade together, each has specific characteristics:

#### Gateway Image Upgrade (API Version Dependent)
- **State**: **Stateless** - Gateway containers have no persistent state
- **Impact**: Medium (rolling restart of pods when API version changes)
- **Risk**: Low - No data loss risk, only temporary connection disruption
- **Risk Mitigation**: Multiple standby instances with local HA ensure zero-downtime rolling restart; Gateway and PostgreSQL containers run in same pod, sharing HA benefits
- **Version Behavior**: Gateway features may differ between cluster API v1 and v2

#### PostgreSQL Database Upgrade (API Version Dependent)  
- **State**: **Stateful** - PostgreSQL contains persistent application data
- **Impact**: Variable (depends on API version differences)
- **Risk**: Variable - Data migration only required for breaking schema changes
- **Risk Mitigation**: CNPG managed HA with supervised rolling updates for zero-downtime upgrades
- **HA Strategy**: 3-instance clusters (1 primary + 2 standby servers) with CNPG-controlled failover sequence
- **Categories**:
  - **Same PostgreSQL Version**: Cluster API v1 → v2 with same PG version (low risk, configuration change only)
  - **Minor PostgreSQL Version**: Different PG minor versions between API versions (medium risk, CNPG rolling restart with automatic switchover)
  - **Major PostgreSQL Version**: Different PG major versions between API versions (high risk, data migration required with blue-green procedures)

#### DocumentDB Postgres Extension Upgrade (API Version Dependent)
- **State**: **Stateful** - Extension may have API-specific features
- **Impact**: Variable (depends on extension differences between API versions)
- **Risk**: Variable - Extension schema changes only if API versions require different features
- **Risk Mitigation**: Extension compatibility testing during operator upgrade; rollback capability maintains previous extension versions if needed
- **Categories**:
  - **Compatible Extension**: Same extension version for both API versions (low risk)
  - **Enhanced Extension**: New features added for v2 API (medium risk, backward compatible)
  - **Breaking Extension Changes**: Schema modifications required for v2 API (high risk, requires migration planning)

#### Sidecar Injector Upgrade (Supports Multiple API Versions)
- **State**: **Stateless** - Injection webhook has no persistent state
- **Impact**: Medium (affects new pod creation, must support both API versions)
- **Risk**: Medium - Injection failures affect new PostgreSQL pods
- **Risk Mitigation**: Multi-version support ensures existing pods continue running; new pod creation uses appropriate API version configuration
- **Multi-Version Support**: Injector must handle both v1 and v2 cluster configurations

#### CNPG Operator Upgrade (API Version Independent)
- **Trigger**: Upgrade bundled with DocumentDB operator when CNPG version needs updating
- **Scope**: Control plane and data plane
- **Impact**: Variable (depends on CNPG upgrade requirements)
- **Risk Mitigation**: CNPG rolling updates maintain cluster availability; proven PostgreSQL HA mechanisms ensure data safety
- **API Independence**: CNPG typically unchanged between DocumentDB API versions

## Local High Availability (HA) Strategy

DocumentDB leverages CNPG's mature PostgreSQL HA capabilities to provide zero-downtime upgrades through controlled failover orchestration.

#### Recommended HA Configuration

**3-Instance Cluster Topology:**
- **Instance count**: 3 (1 primary + 2 standby servers) for optimal HA balance
- **Primary update strategy**: Supervised for production (manual controlled switchover), unsupervised for development
- **Planned switchover**: Manually initiated using CNPG promote command while in supervised mode
- **Unplanned failover**: Handled automatically by CNPG (no custom delay fields required)
- **PostgreSQL configuration**: Streaming replication with quorum synchronous replication (e.g., synchronous_standby_names='ANY 1 (*)', synchronous_commit=remote_write) to balance durability and availability

**Configuration Examples**: See [CNPG HA Configuration Examples](./commands.md#cnpg-ha-configuration-examples) for complete YAML specifications.

#### Zero-Downtime Upgrade Sequence

**CNPG Managed Rolling Update Process:**

1. **Standby Server Upgrade Phase** (Automatic):
   - CNPG upgrades standby instances first (highest serial number to lowest)
   - Each standby server downloads new images and restarts with new configuration
   - Primary continues serving traffic with full availability

2. **Controlled Switchover Phase** (Manual with Supervised Mode):
   - Check standby lag before switchover to ensure optimal timing
   - Manual switchover to most aligned standby server using CNPG promote command
   - Service endpoints automatically update to new primary

3. **Primary Upgrade Phase** (Automatic):
   - Former primary becomes standby server and receives upgrade
   - New primary (former standby server) continues serving traffic
   - Zero service interruption during transition

**Command Examples**: See [CNPG Zero-Downtime Upgrade Sequence](./commands.md#cnpg-zero-downtime-upgrade-sequence) for step-by-step commands.

#### Operational Benefits of CNPG HA

**✅ Built-in Capabilities:**
- **WAL-based replication**: Ensures data consistency during failover between primary and standby servers
- **Automatic endpoint management**: DNS and service updates during switchover
- **Connection draining**: Graceful client connection handling
- **Monitoring integration**: Real-time standby lag and health metrics
- **Rollback support**: Can revert primary assignment if issues detected

**⚠️ Operational Considerations:**
- **Supervised mode**: Production environments use manual switchover for maximum control
- **Standby lag monitoring**: Check lag before manual switchover for optimal timing
- **Connection pool awareness**: Applications should use read/write service endpoints for seamless failover
- **Monitoring integration**: Real-time health checks help inform manual switchover decisions


#### Risk Mitigation Enhancements

**Enhanced with CNPG Specifics:**
- **Automatic failover**: Unplanned failures trigger CNPG-managed failover (no custom failoverDelay field required)
- **Planned maintenance**: Supervised upgrades allow controlled, low-lag switchover timing
- **Data protection**: WAL streaming plus quorum synchronous replication mitigate data loss while avoiding write stalls from a single replica outage
- **Service continuity**: Kubernetes service endpoints automatically update during failover/switchover
- **Monitoring integration**: CNPG status and pg_stat_replication provide real-time health & lag metrics

This CNPG-based HA strategy ensures DocumentDB clusters achieve true zero-downtime upgrades while maintaining data integrity and operational simplicity.

## Multi-Node Upgrade Strategy (Future Enhancement)

**Multi-Node Upgrade Considerations**: While this document focuses on single-node DocumentDB clusters with local HA (1 primary + 2 standby servers per node as an example), future multi-node deployments will support horizontal scaling using multiple PostgreSQL clusters managed by Citus. This document provides some high level idea and the detials will be covered in a separate multi-node upgrade design doc.

### Citus Integration for Multi-Node Architecture

DocumentDB will leverage **Citus** (PostgreSQL extension for distributed SQL) to enable horizontal scaling through multi-node deployments. This integration requires enhancements to the CNPG operator to support Citus-specific cluster topologies.

**Multi-Node Architecture with Citus**

The Citus cluster architecture consists of a single coordinator node that manages distributed queries and metadata, along with multiple worker nodes that provide horizontal scaling capabilities. Each node maintains its own high availability configuration, with the coordinator node running 1 primary plus 2 standby servers managed by CNPG, and each worker node following the same HA pattern. For example, a deployment with 3 worker nodes would result in 9 total PostgreSQL instances (3 primaries and 6 standby servers). Citus MX handles intelligent query routing and distributed transaction coordination across all nodes, while the sharding strategy automatically distributes data across worker nodes based on document keys.

**CNPG Integration Requirements**

To support this architecture, CNPG will need several enhancements including cluster-level configuration support for Citus coordinator and worker node specifications, enhanced service discovery for inter-node communication, coordinated backup procedures across all Citus nodes, and orchestrated upgrades that maintain Citus cluster consistency throughout the process.

**Upgrade Complexity Considerations**

Multi-node upgrades introduce significant complexity that requires careful consideration of node upgrade sequencing based on availability zones, traffic balancing across worker nodes, cross-node dependency analysis, and risk mitigation strategies such as upgrading non-critical worker nodes before the coordinator node. The primary orchestration challenges include synchronizing upgrades across distributed worker nodes, maintaining data consistency during worker node upgrades, handling partial upgrade failures across multiple nodes, coordinating Citus metadata updates during upgrades, and ensuring Citus MX routing remains functional throughout the upgrade process.

**Citus-Specific Upgrade Strategy**

The upgrade strategy must account for several Citus-specific considerations. The sequencing strategy needs to determine whether to upgrade worker nodes first or the coordinator first, based on Citus version compatibility requirements. Metadata synchronization becomes critical to ensure Citus metadata consistency during rolling upgrades across nodes. Shard rebalancing must be coordinated during worker node upgrades to manage data redistribution effectively. Additionally, maintaining Citus MX routing functionality during node transitions and managing inter-node connectivity during upgrade phases are essential for seamless operations.

## Multi-Region Upgrade Strategy (Future Enhancement)

**Multi-Region Upgrade Considerations**: While multi-node deployments focus on horizontal scaling within a single location, future multi-region deployments will address geographic distribution challenges across different regions, clouds, or data centers using a primary-replica region architecture. Multi-region upgrades introduce additional complexity including cross-region network latency considerations, provider-specific maintenance windows, data sovereignty and compliance requirements, regional disaster recovery coordination, and potential split-brain scenarios during network partitions between regions. The orchestration strategy will need to account for replica-first upgrade sequencing (upgrading replica regions before the primary region), cross-region data consistency validation between primary and replica regions, region-specific rollback procedures, and coordinated monitoring across geographically distributed infrastructure. This multi-region upgrade strategy will be addressed in a dedicated design document when DocumentDB expands beyond single-region deployments.



## Failure Modes and Recovery (Essentials)

Focus on highest-impact, actionable scenarios only.

1. Helm upgrade / CRD change fails
   - Impact: New CR creation blocked; existing clusters keep running
   - Detect: Helm timeout/errors
   - Recover: Automatic rollback (`--atomic`); if partial, delete new CRD + redeploy previous chart
   - Prevent: Mandatory dry-run + test upgrade in lower envs

2. Operator crash loop after upgrade
   - Impact: Reconciliation paused; running clusters unaffected
   - Detect: Pod restart loop / liveness probe failures
   - Recover: Rollback chart; inspect logs/resources; adjust limits
   - Prevent: Resource limits + readiness/liveness tuned

3. Cluster API migration fails
   - Impact: Single cluster degraded
   - Detect: Ready condition not true within window / errors in status
   - Recover: Reapply previous manifest (apiVersion/spec); restore from backup if corruption
   - Prevent: Pre-migration backup + health/lag checks + dry-run patch

4. Gateway container fails on new API
   - Impact: App connectivity loss (one cluster)
   - Detect: CrashLoopBackOff / failed health probes
   - Recover: Roll back gateway image; restart pod
   - Prevent: Canary migration + image compatibility tests

5. Network partition / replica divergence risk
   - Impact: Potential replication lag or write blockage (split-brain prevented by single primary design)
   - Detect: Elevated replication lag; unexpected role changes; timeline anomalies
   - Recover: Allow CNPG failover; if divergence suspected, promote known-good standby and reattach others
   - Prevent: Stable network, monitoring lag & role labels

6. Concurrent migrations overload resources
   - Impact: Multiple clusters slow / fail
   - Detect: Resource saturation (CPU/memory), multiple Ready timeouts
   - Recover: Pause further migrations; rollback affected clusters
   - Prevent: Throttle batch size; enforce migration schedule

Rollback Golden Rules:
- Always have recent logical/physical backup before Phase 2
- Automate fast rollback for stateless/operator issues; keep manual confirmation for data changes
- Track each migration (cluster, from→to, start/finish, result) for audit

## Trade-off Analysis

- Unified Component Version vs Independent Component Versions: A single unified version simplifies operations and guarantees compatibility, at the cost of less flexibility to patch components independently.
- Multi-Version API Support vs Single Version Enforcement: Serving multiple API versions concurrently enables gradual cluster migration and reduces pressure on teams, while increasing operator complexity and test scope.
- Rolling Upgrades vs Always Blue-Green: Rolling upgrades minimize extra infrastructure and networking overhead, accepting slightly higher in-place change risk; blue‑green is reserved for exceptional high‑risk cases.
- Hybrid Rollback Strategy vs Full Automation: Automating rollbacks for stateless pieces and keeping manual judgment for stateful data preserves data safety while adding a small decision delay.
- Role Separation (K8s Admin / DBA / App Dev) vs Single Owning Team: Splitting responsibilities aligns expertise and allows parallel work, introducing some coordination overhead.

## Implementation Reference

For detailed command examples, scripts, and operational procedures, see:

**[Command Reference Guide](./commands.md)** - Complete command examples for:
- Multi-version API workflow commands
- Infrastructure upgrade procedures
- Cluster API migration examples
- CNPG supervised HA upgrade procedures
- Rollback and emergency procedures
- Component hash tracking scripts
- Blue-green deployment procedures