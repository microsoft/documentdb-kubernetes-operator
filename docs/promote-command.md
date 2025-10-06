# `kubectl documentdb promote` Design Notes

## Summary
`kubectl documentdb promote` elevates a chosen member cluster to become the new primary for a DocumentDB fleet deployment. The command updates the hub-side `DocumentDB` custom resource, then waits until the fleet reports that the new primary is live and healthy.

## Usage
```bash
kubectl documentdb promote \
  --documentdb <name> \
  --namespace <ns> \
  --target-cluster <fleet-member> \
  [--hub-context <kubecontext>] \
  [--cluster-context <kubecontext>] \
  [--wait-timeout 10m] \
  [--poll-interval 10s] \
  [--skip-wait]
```

**Key flags**
- `--documentdb` *(required)* – DocumentDB resource name.
- `--namespace` – Namespace hosting the resource. Defaults to `default`.
- `--target-cluster` *(required)* – Fleet member that should become primary.
- `--hub-context` – Explicit kubeconfig context for the fleet hub. Falls back to the current context when omitted.
- `--cluster-context` – Member context used for validation. Defaults to the target cluster name.
- `--skip-wait` – Submit the promotion and exit immediately, without verifying convergence.
- `--force` – Bypass the pre-flight health check on the target cluster (use with caution).

## Control Flow
1. **Parse & validate input**
  - Cobra parses flags into `promoteOptions`.
  - `complete()` clamps `wait-timeout` / `poll-interval` to sensible defaults whenever the user supplies zero or negative values.
  - Cobra enforces required flags (`--documentdb`, `--target-cluster`) before `RunE` executes.
2. **Resolve hub configuration**
  - `loadConfig(--hub-context)` reads the user’s kubeconfig and optionally forces a specific context via overrides.
  - Missing contexts throw `kubeconfig context <name> not found`; the command exits before touching the API.
  - Returns the REST config and resolved context name, which doubles as the default for `--cluster-context` and is echoed back to the user.
3. **Derive target configuration**
  - If `--cluster-context` is omitted, default to the `--target-cluster` name (assuming kube contexts follow the fleet naming convention).
  - `loadConfig(--cluster-context)` produces another REST config and context string whenever a target context is required (either because `--skip-wait` is false or `--force` is not set).
  - Build a second `dynamic.Interface` (`dynTarget`) for member polling and/or health checks.
  - Pre-flight guard: when `--force` is **not** set, fetch the target DocumentDB and ensure it reports a healthy status. Fail fast with guidance when the target cluster is missing or unhealthy.
4. **Patch DocumentDB on the hub**
  - Build a JSON merge payload: `{ "spec": { "clusterReplication": { "primary": <targetCluster> } } }`.
  - Execute `dynHub.Resource(gvr).Namespace(ns).Patch(..., types.MergePatchType, patchBytes, ...)` to update only the `primary` field.
  - Any API failure surfaces immediately as `failed to patch DocumentDB <name>` and aborts the command.
5. **Optional wait loop** (skipped when `--skip-wait` is supplied)
  - Start a context with timeout (`--wait-timeout`, default 10m) and a ticker (`--poll-interval`, default 10s).
  - **Hub probe**
    1. Fetch the authoritative DocumentDB from the hub using `dynHub`.
    2. Run `isDocumentReady(hubDoc, targetCluster)`:
      - Assert `spec.clusterReplication.primary == targetCluster`.
      - Inspect `status.status`; treat empty as healthy, otherwise accept values `healthy`, `ready`, `running`, `succeeded`, or strings containing “healthy/ready”.
    3. If the hub copy isn’t ready, sleep until the next tick.
  - **Target probe** (only when a target context exists)
    1. Fetch the member copy via `dynTarget`.
    2. If the resource is missing (`IsNotFound`), continue polling.
    3. Re-run `isDocumentReady` to ensure the member sees the right primary and reports a healthy status.
  - Loop until both probes succeed or the timeout expires.
  - On timeout the command returns `timed out waiting for promotion to complete after <duration>`—the spec patch already succeeded, so follow-up inspection is recommended.
6. **Success reporting**
  - Print “Promotion completed successfully.” and exit 0.

Throughout, the hub DocumentDB serves as the source of truth for desired state, while the target member check confirms propagation across the fleet.

## Error Handling & Diagnostics
- **Kubeconfig loading failures** – missing or misspelled contexts emit a clear `kubeconfig context <name> not found` error before any changes occur.
- **Patch failures** – HTTP errors from the hub API return immediately with `failed to patch DocumentDB <name>`.
- **Propagation timeout** – if the fleet has not converged by `--wait-timeout`, the command returns `timed out waiting for promotion to complete…` so operators know the request succeeded but rollout is lagging.
- **Member fetch errors** – unexpected API errors while polling include the context and resource name to simplify debugging.

## Operational Notes
- The command relies on the updated binary at `plugins/bin/kubectl-documentdb`; ensure the directory precedes other `kubectl-documentdb` binaries on `$PATH`.
- For live progress, pair the command with hub-side `kubectl describe documentdb <name>` or operator log streaming to monitor status and events during the promotion.
- The companion command `kubectl documentdb events --documentdb <name> --namespace <ns> [--hub-context hub]` streams the Kubernetes events emitted for the DocumentDB resource, making it easy to watch promotions that outlast the CLI wait timeout.
