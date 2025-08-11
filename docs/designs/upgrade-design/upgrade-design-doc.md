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
An admission webhook that automatically injects the Gateway container into CNPG PostgreSQL pods during deployment. Runs in the `cnpg-system` namespace on worker nodes.

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
Support N-2 API versions with 6-month deprecation cycles for gradual migration.

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
- **Real-time migration**: API migrations designed for planned execution windows
- **Multi-tenant upgrades**: Each DocumentDB cluster upgraded independently

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

## Upgrade Scenarios

**With Unified Versioning**: All component upgrades are triggered by a single DocumentDB operator version upgrade. Individual component upgrades are not available to customers.

## Team Responsibilities and Multi-Version Support Strategy

DocumentDB uses a **multi-version API approach** where a single operator version supports multiple DocumentDB cluster versions simultaneously, enabling gradual migration without forcing upgrades.

### Team Roles and Responsibilities

**Three-Tier Responsibility Model:**

#### 1. Kubernetes Admin / Control Plane Admin
- **Scope**: Kubernetes cluster infrastructure and operators
- **Responsibilities**: 
  - Kubernetes cluster upgrades and maintenance
  - Operator lifecycle management (DocumentDB, CNPG, Sidecar Injector)
  - Infrastructure-level component upgrades
  - Platform security and resource management
- **Upgrade Activities**: Infrastructure upgrade phase (operator components)

#### 2. Database Admin (DBA)
- **Scope**: DocumentDB instance management across environments
- **Responsibilities**:
  - DocumentDB cluster provisioning for dev/test/staging/production
  - Database configuration and version rollouts
  - Cluster API version migration planning and coordination
  - Performance monitoring and capacity planning
  - Backup and disaster recovery strategies
- **Upgrade Activities**: Cluster API migration coordination and execution

#### 3. Database Developer / Application Developer
- **Scope**: Database data and application integration
- **Responsibilities**:
  - Application database schema and data management
  - Client application integration with DocumentDB clusters
  - Test coverage across development environments
  - Application-specific migration testing and validation
- **Upgrade Activities**: Application compatibility testing and validation

### Multi-Version Support Architecture

**Single Operator, Multiple Cluster Versions:**
- **Operator v2**: Supports both DocumentDB cluster `v1` and `v2` APIs
- **Operator v3**: Supports DocumentDB cluster `v1` (deprecated), `v2`, and `v3` APIs  
- **Operator v4**: Supports DocumentDB cluster `v2`, `v3`, and `v4` APIs (v1 removed)

**API Deprecation Cycle:**
- **Version N**: Introduce new cluster API version
- **Version N+1**: Previous version marked as deprecated but still supported
- **Version N+2**: Deprecated version removed, only latest 2-3 versions supported

### Upgrade Process with Multi-Version Support

### Phase 1: Infrastructure Upgrade (Kubernetes Admin)
**Responsibility**: Upgrade DocumentDB operator infrastructure to support new cluster versions
**Scope**: Kubernetes-level operator components only
**Command**: Helm upgrade of DocumentDB operator chart

**What gets upgraded**:
- DocumentDB Operator controller (v1 → v2) - now supports both cluster v1 and v2
- CNPG Operator (if version change required)
- Sidecar Injector (v1 → v2) 
- CRDs with new v2 fields added, v1 fields maintained
- RBAC, and operator configurations

**What does NOT get upgraded**:
- Existing DocumentDB clusters remain on v1 API
- Applications continue using v1 DocumentDB features unchanged
- No data migration or application downtime
- **Backward Compatibility**: v2 operator fully supports existing v1 clusters

### Phase 2: Cluster Migration Coordination (Database Admin)  
**Responsibility**: Plan and coordinate DocumentDB cluster API version migrations
**Scope**: Per-cluster API version upgrades across environments
**Method**: Coordinate with Database Developers for phased migration execution

**What gets planned and coordinated**:
- Cluster API migration timeline across dev/test/staging/production
- Migration dependencies and environment-specific requirements
- Database configuration updates for v2 API features
- Cluster-level backup and validation procedures
- Performance and capacity impact assessment

**Database Admin Control**:
- Plans migration schedule across all environments
- Coordinates with Database Developers for execution
- Manages cluster configurations and database-level settings
- Oversees backup and recovery procedures during migration
### Phase 3: Application Migration Execution (Database Developer)
**Responsibility**: Execute cluster API migration and validate application compatibility
**Scope**: Application-specific testing and validation
**Method**: Update API version in deployment files and execute migration with comprehensive testing

**What gets executed**:
- Specific DocumentDB cluster API (v1 → v2) migration
- Application compatibility testing with v2 API features
- Client application integration validation
- Data integrity verification during migration
- Performance testing and optimization

**Database Developer Control**:
- Executes migration based on Database Admin's coordination plan
- Gradual rollout across development, staging, production environments
- Application-specific testing and validation with v2 features
- Rollback capability per cluster (v2 → v1 downgrade) if application issues occur
- Test coverage across environments to ensure application compatibility

**Benefits of Three-Tier Responsibility Model**:
- **Clear Role Separation**: Each team focuses on their domain expertise
- **Coordinated Migration**: Database Admins coordinate while Database Developers execute
- **Gradual API Migration**: Database Developers control application-specific migration timeline
- **Risk Mitigation**: Test new API versions in dev/staging before production migration
- **Backward Compatibility**: Multiple cluster API versions supported simultaneously
- **Controlled Deprecation**: API versions deprecated gradually over multiple operator releases

**Cross-Team Coordination**: Database Admins coordinate migration planning with Database Developers for execution, while Kubernetes Admins provide the infrastructure foundation.

**API Version Examples**: See [commands.md](./commands.md) for detailed workflow commands and API version migration examples.

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
- **Primary update strategy**: Supervised for production, unsupervised for development
- **Primary update method**: Switchover for planned failover to standby server during upgrades
- **Switchover delay**: 30-second graceful shutdown timeout for planned upgrades
- **Failover delay**: Immediate failover (0s) for unexpected failures
- **PostgreSQL configuration**: Streaming replication with synchronous commit enabled (for standby server synchronization)

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
- **Automatic failover**: Unplanned failures trigger immediate CNPG failover (`.spec.failoverDelay: "0s"`)
- **Planned maintenance**: Supervised upgrades allow optimal timing for switchover
- **Data protection**: WAL streaming and synchronous replication prevent data loss between primary and standby servers
- **Service continuity**: Kubernetes service endpoints automatically update during failover
- **Monitoring integration**: CNPG status commands provide real-time cluster health visibility

This CNPG-based HA strategy ensures DocumentDB clusters achieve true zero-downtime upgrades while maintaining data integrity and operational simplicity.

## Multi-Node Upgrade Strategy (Future Enhancement)

**Multi-Node Upgrade Considerations**: While this document focuses on single-node DocumentDB clusters with local HA (1 primary + 2 standby servers per node), future multi-node deployments will support geographic distribution where each node runs an independent DocumentDB cluster. Multi-node upgrades will require careful consideration of node upgrade sequencing based on geographic distribution and availability zones, traffic balancing across nodes, cross-node dependency analysis, and risk mitigation by upgrading non-critical nodes first. Additional complexity arises in multi-cloud scenarios involving cross-cloud networking, provider-specific maintenance windows, data sovereignty requirements, and coordinated monitoring across cloud providers. The orchestration challenges include synchronizing upgrades across geographically distributed clusters, maintaining global data consistency, handling partial upgrade failures across multiple nodes/clouds, and coordinating teams across different regions. This multi-node upgrade strategy will be covered in a separate design document when DocumentDB enables multi-node deployment scenarios, building upon the current single-node HA strategy as the foundation.

## Upgrade Strategies

**Multi-Version Support Approach**: DocumentDB uses a multi-version API strategy where a single operator version supports multiple DocumentDB cluster API versions simultaneously, enabling controlled migration.

### 1. Infrastructure Upgrade Strategy (Kubernetes Admin)

The operator infrastructure upgrade involves upgrading the control plane components while leaving existing DocumentDB clusters unchanged.

#### A. Infrastructure Components Upgrade

**Components Upgraded in Infrastructure Phase:**
- **DocumentDB Operator**: Controller with multi-version API support, webhooks, CRDs
- **Sidecar Injector**: Container injection webhook supporting multiple cluster API versions
- **CNPG Operator**: PostgreSQL cluster management (when version updates required)

**Components NOT Upgraded in Infrastructure Phase:**
- **DocumentDB Clusters**: Remain on current API version until Phase 2 migration
- **Gateway Images**: Stay at current version until API migration
- **PostgreSQL + Extension**: No changes to running databases until API migration

**Version Alignment Example (Infrastructure Phase):**
```yaml
# After Infrastructure Phase: Operator v2 supports cluster API v1 and v2
documentdb-operator: v2.0.0          # ✅ Upgraded (supports cluster API v1 + v2)
sidecar-injector: v2.0.0             # ✅ Upgraded (supports cluster API v1 + v2)
cnpg-operator: v0.26.0               # ✅ Upgraded (if required)
# Existing clusters remain on API v1
cluster-api-version: v1              # ⏸️ Not migrated yet
cluster-gateway: cluster-api-v1      # ⏸️ Not migrated yet  
cluster-postgres: cluster-api-v1     # ⏸️ Not migrated yet
```

#### B. Infrastructure Upgrade Process

**Single Helm Upgrade Approach (Kubernetes Admin):**

1. **Pre-upgrade validation** using Helm dry-run to identify potential issues
2. **Atomic upgrade execution** with rollback on failure using Helm's `--atomic` flag  
3. **Operator health verification** ensuring all operators reach ready state
4. **Multi-version compatibility check** ensuring v2 operator can manage both v1 and v2 cluster APIs

**Command Examples**: See [commands.md](./commands.md) for detailed Infrastructure Upgrade commands and validation.

**Upgrade Process Flow (Infrastructure Phase):**
1. **CNPG Operator** (if version update required)
2. **DocumentDB Operator** (CRDs with v2 fields, controller with multi-version support, webhooks, RBAC)
3. **Sidecar Injector** (webhook supporting both cluster API versions)
4. **Validation** that v2 operator can manage existing v1 clusters and create new v2 clusters

### 2. Cluster API Migration Strategy (Database Admin + Database Developer)

The cluster API migration involves coordinated planning by Database Admins and execution by Database Developers for transitioning individual DocumentDB clusters from v1 to v2 API.

#### A. Per-Cluster API Migration Components

**Components Migrated in API Migration (per cluster):**
- **Cluster API Schema**: DocumentDB cluster configuration migrated from v1 to v2 fields
- **Gateway Image**: Updated to support v2 API features (if different from v1)
- **PostgreSQL + Extension**: Updated to support v2 API features (if different from v1)
- **Cluster Configuration**: Access to new v2 features and APIs

**Database Admin + Database Developer Coordinated Process:**
- **Migration Planning**: Database Admin coordinates timeline across environments  
- **Migration Execution**: Database Developer executes per Database Admin's plan
- **Feature Testing**: Database Developer tests v2 API features before production migration
- **Rollback Capability**: Individual cluster API version downgrade (v2 → v1) if needed

**Command Examples**: See [commands.md](./commands.md) for detailed Database Admin coordination and Database Developer execution commands.

#### B. Cluster API Migration Process

**Developer-Initiated Commands:**
See [commands.md](./commands.md) for detailed cluster API migration commands including backup, migration, monitoring, and rollback procedures.

**Cluster API Migration Process Flow:**
1. **Pre-migration backup** (if required for significant changes)
2. **API schema migration** from v1 to v2 fields and configuration  
3. **Rolling update** of PostgreSQL pods with v2-compatible images (if needed)
4. **Gateway container restart** with v2 API support (if needed)
5. **Configuration validation** of cluster functionality with v2 API
6. **Application testing** with v2 API features

### 3. Multi-Version API Support Examples

#### Example 1: Operator v2 Supporting Cluster API v1 and v2
**Infrastructure Phase (Kubernetes Admin):**
Operator infrastructure upgrade: v1 → v2
- Operator components: v1 → v2 (now supports cluster API v1 + v2)
- Existing clusters: remain on cluster API v1
- New clusters: can be created with either cluster API v1 or v2

**API Migration Phase (Database Admin coordination + Database Developer execution):**
Database Admin coordinates migration plan, Database Developer executes cluster API version migration from v1 to v2

**API Coexistence**: Operator v2 manages both cluster API v1 and v2 simultaneously

**Command Examples**: See [commands.md](./commands.md) for detailed multi-version API commands.

#### Example 2: Operator v3 with API Deprecation (Medium Risk)
**Infrastructure Phase (Kubernetes Admin):**
Operator infrastructure upgrade: v2 → v3
- Operator components: v2 → v3 (supports cluster API v1-deprecated, v2, v3)
- Cluster API v1: marked as deprecated but still functional
- Existing clusters: all versions continue running unchanged

**API Migration Phase (Database Admin coordination + Database Developer execution):**
Week 1: Development clusters (v1 → v2 or v1 → v3)
Week 2: Staging validation
Week 3: Production (after testing new API versions)

**Command Examples**: See [commands.md](./commands.md) for detailed API deprecation migration commands.

#### Example 3: Operator v4 with API Removal (High Risk)
**Infrastructure Phase (Kubernetes Admin):**
Operator infrastructure upgrade: v3 → v4
- All operator components: v3 → v4 (supports cluster API v2, v3, v4)
- Cluster API v1: removed (no longer supported)
- **Prerequisites**: All clusters must be migrated off cluster API v1 before operator upgrade

**API Migration Phase (Database Admin coordination + Database Developer execution):**
Month 1: Development clusters (v2 → v3 or v2 → v4)
Month 2: Staging environment validation
Month 3: Production (after extensive testing)

**Command Examples**: See [commands.md](./commands.md) for detailed API removal migration commands.

## Failure Modes and Recovery

### Operator Infrastructure Failures

#### Scenario: Helm Upgrade Fails During CRD Update
**Impact**: New clusters cannot be created, existing clusters unaffected
**Probability**: Medium (complex CRD schema changes)
**Detection**: Helm upgrade timeout or validation errors
**Recovery**: 
- Automatic Helm rollback via `--atomic` flag
- Manual CRD cleanup if needed: `kubectl delete crd documentdbs.db.microsoft.com`
- Re-apply previous operator version
**Prevention**: 
- Mandatory Helm dry-run validation before upgrade
- Staged rollouts in non-production environments first
- CRD schema compatibility testing in CI/CD

#### Scenario: DocumentDB Operator Pod Crash During Upgrade
**Impact**: Existing clusters stable, new cluster creation blocked
**Probability**: Low (robust health checks)
**Detection**: Pod restart loops, operator health check failures
**Recovery**:
- Kubernetes restarts operator pod automatically
- If persistent failure, Helm rollback to previous version
- Check resource limits and node capacity
**Prevention**:
- Resource requests/limits properly configured
- Health checks with appropriate timeouts
- Pod disruption budgets prevent simultaneous restarts

### API Migration Failures

#### Scenario: Cluster API Migration Fails Mid-Process
**Impact**: Single cluster affected, others continue operating normally
**Probability**: Low (pre-migration validation)
**Detection**: API migration timeout, cluster status degradation
**Recovery**:
- Per-cluster rollback to previous API version
- `kubectl patch documentdb cluster-name --type='merge' -p '{"apiVersion": "db.microsoft.com/v1"}'`
- Restore from pre-migration backup if data corruption
**Prevention**:
- Mandatory backup before API migration
- Pre-migration cluster health validation
- Staged migration (dev → staging → production)

#### Scenario: Gateway Container Fails to Start with New API Version
**Impact**: MongoDB connectivity lost for single cluster
**Probability**: Medium (configuration incompatibilities)
**Detection**: Pod crash loops, connection test failures
**Recovery**:
- Rolling restart of PostgreSQL pods
- Revert to previous gateway image version
- Manual configuration correction if needed
**Prevention**:
- Container image compatibility testing
- Canary deployment for new gateway versions
- Comprehensive integration test suite

### Split-Brain and Consistency Failures

#### Scenario: Network Partition During Rolling Upgrade
**Impact**: Potential data inconsistency between standby servers
**Probability**: Low (robust network infrastructure)
**Detection**: CNPG cluster status reports split-brain condition
**Recovery**:
- CNPG automatic recovery mechanisms engage
- Manual intervention for prolonged partitions
- Restore from backup if data corruption detected
**Prevention**:
- Extended timeout windows for network instability
- Proper health checks with retry logic
- Network monitoring and alerting

#### Scenario: Concurrent API Migrations Cause Resource Conflicts
**Impact**: Multiple clusters fail migration simultaneously
**Probability**: Very Low (developer coordination)
**Detection**: Resource exhaustion, multiple cluster failures
**Recovery**:
- Throttle concurrent migrations
- Prioritize critical production clusters
- Staged rollback of failed migrations
**Prevention**:
- API migration coordination guidelines
- Resource capacity planning
- Automated migration scheduling

## Trade-off Analysis

This section analyzes key architectural decisions where we had to choose between competing approaches. Each trade-off explains the alternatives considered and why we selected our approach.

### Multi-Version API Support vs Single Version Enforcement
**The Choice**: Support 2-3 API versions simultaneously in single operator
**Alternative Rejected**: Force all clusters to upgrade to latest API version immediately
**Trade-offs**:
- **Choosing Multi-Version Support**:
  - ✅ **Benefit**: 6-month migration windows, 90% reduction in forced upgrade incidents
  - ❌ **Cost**: ~30% increase in operator codebase size, additional testing matrix
- **Alternative (Single Version)**:
  - ✅ **Benefit**: Simpler codebase, single testing path
  - ❌ **Cost**: Breaking changes force immediate migrations, higher operational risk
**Decision**: Accept complexity to enable gradual migrations (customer requirement)

### Unified Versioning vs Component Independence  
**The Choice**: All components versioned together with single release
**Alternative Rejected**: Independent versioning for each component (operator, gateway, postgres, etc.)
**Trade-offs**:
- **Choosing Unified Versioning**:
  - ✅ **Benefit**: Single version to track, eliminates version matrix compatibility testing
  - ❌ **Cost**: Larger upgrade surface area, more components change per upgrade
- **Alternative (Independent Versioning)**:
  - ✅ **Benefit**: Granular control, smaller upgrade scope per component
  - ❌ **Cost**: Complex version matrix (5 components × multiple versions), compatibility hell
**Decision**: Prioritize operational simplicity over granular control

### Rolling vs Blue-Green Upgrades
**The Choice**: Rolling upgrades as default, blue-green for major versions only
**Alternative Rejected**: Blue-green deployments for all upgrades
**Trade-offs**:
- **Choosing Rolling Upgrades**:
  - ✅ **Benefit**: Uses existing capacity, saves ~50% infrastructure costs
  - ❌ **Cost**: Higher failure rate (0.1% vs 0.01%), temporary service degradation
- **Alternative (Blue-Green Only)**:
  - ✅ **Benefit**: Near-zero downtime, instant rollback capability
  - ❌ **Cost**: Requires 2x resources, complex networking setup
**Decision**: Use rolling for cost efficiency, blue-green only for high-risk scenarios

### Automatic vs Manual Rollbacks
**The Choice**: Automatic rollback for infrastructure, manual approval for data plane
**Alternative Rejected**: Fully automatic rollbacks for all components
**Trade-offs**:
- **Choosing Hybrid Approach**:
  - ✅ **Benefit**: Fast recovery for infrastructure (95% faster), human oversight for data
  - ❌ **Cost**: Requires on-call engineering judgment for data plane issues
- **Alternative (Fully Automatic)**:
  - ✅ **Benefit**: Fastest possible recovery, no human intervention needed
  - ❌ **Cost**: Risk of automatic rollback making data corruption worse
**Decision**: Automatic for stateless, manual approval for stateful components

### Team Autonomy vs Centralized Control
**The Choice**: Split responsibility between Kubernetes admins (infrastructure), Database admins (coordination), and Database developers (execution)
**Alternative Rejected**: Single team controls all upgrade phases
**Trade-offs**:
- **Choosing Three-Tier Responsibility Model**:
  - ✅ **Benefit**: Domain expertise alignment, coordinated execution, reduced bottlenecks across teams
  - ❌ **Cost**: Requires coordination between Database admins and Database developers
- **Alternative (Centralized Control)**:
  - ✅ **Benefit**: Single point of responsibility, consistent upgrade process
  - ❌ **Cost**: Bottlenecks on single team, slower overall upgrade velocity, domain expertise dilution
**Decision**: Accept coordination overhead for improved domain alignment and team velocity

### CNPG Supervised vs Unsupervised Upgrades
**The Choice**: Hybrid approach - unsupervised for development, supervised for production
**Alternative Rejected**: Fully automatic unsupervised upgrades for all environments
**Trade-offs**:
- **Choosing Hybrid Approach**:
  - ✅ **Benefit**: Fast automated dev/staging, manual control for production safety
  - ❌ **Cost**: Requires manual intervention (~1-5 minutes), environment-specific procedures
- **Alternative (Unsupervised Only)**:
  - ✅ **Benefit**: Fully automated, no human intervention needed
  - ❌ **Cost**: Higher risk of unexpected production issues during failover
**Decision**: Use unsupervised for development, supervised manual control for production

---

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