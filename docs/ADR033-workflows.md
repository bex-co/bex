# ADR033 — Workflows: Temporal orchestration over isolated task Jobs

**Status:** Proposed, off-roadmap. Workflows remain an explicit non-goal in `.pm/DO_NOT_DO.md`; this ADR records the design to use if that decision is revisited. It does not authorize implementation.

## Context

Render Workflows exposes durable TypeScript and Python functions. A workflow run may call named tasks, wait for child tasks, retry failures, and survive process restarts. The customer code is untrusted and each task executes in an isolated container.

The public documentation does not name Render's orchestration engine. The available evidence points to Temporal, but this remains an inference rather than a confirmed implementation detail:

| Evidence | Finding |
| --- | --- |
| [Render Workflows docs](https://render.com/docs/workflows) | Tasks are ordinary TypeScript or Python functions and execute in separate containers. |
| [Render TypeScript SDK](https://github.com/renderinc/workflows-sdk/tree/main/packages/render-workflows/src) | The SDK communicates with a local supervisor over a Unix-domain socket. |
| [Render Python SDK](https://github.com/renderinc/workflows-sdk/tree/main/packages/render_workflows/render_workflows) | The Python SDK uses the same local protocol. |
| [Render CLI source](https://github.com/render-oss/cli/blob/main/pkg/workflows/workflows.go) | The workflow client carries Temporal workflow and run IDs and contains Temporal signal code. |

The design must therefore reproduce the observable contract without depending on Render's private implementation. It must also treat arbitrary customer code as hostile. Temporal is an orchestrator, not a sandbox.

## Decision

### 1. Public contract and scope

If Workflows enters the roadmap, bex will implement the Render-compatible contract over a self-hosted [Temporal](https://github.com/temporalio/temporal) control plane. Temporal is an internal mechanism and is never exposed as the customer API.

The first release will support:

- TypeScript and Python SDK compatibility;
- immutable workflow versions and named tasks;
- asynchronous runs, child tasks, retries, cancellation, and logs; and
- at-least-once task execution.

Durable in-process checkpoints, arbitrary language workers, schedules, and a visual workflow editor are deferred.

### 2. Trust boundary and topology

Only trusted bex code runs as a Temporal worker. A trusted orchestration workflow handles opaque workflow, version, run, task, and attempt IDs. It never imports, evaluates, or executes tenant source code.

```text
customer
   |
   v
bex-api ----> control-plane Postgres
   |
   v
trusted bex Temporal worker ----> private Temporal cluster
   |
   v
TaskAttempt CR ----> operator ----> isolated Kubernetes Job
                                      |-- trusted supervisor
                                      `-- untrusted tenant image
```

bex will normally run one shared Temporal cluster per region or failure domain, with separate [Temporal namespaces](https://docs.temporal.io/namespaces) for environments or shards—not one Temporal deployment per tenant. Temporal namespaces and task queues are routing and operational boundaries, not the tenant security boundary.

Tenant workloads receive no Temporal endpoint or credential. The cluster is private, and every Temporal client is trusted platform code authenticated with mTLS and authorized by a non-default [Authorizer](https://docs.temporal.io/self-hosted-guide/security). A dedicated Temporal namespace or cluster may be offered for regulated or dedicated-tenant deployments, but is not required for safe multi-tenancy.

### 3. Durable records and operator boundary

Postgres remains the product source of truth. It stores tenant-scoped workflow definitions, immutable versions, tasks, runs, attempts, inputs, results, and user-visible events. Temporal stores orchestration history and timers, using only opaque resource IDs and bounded metadata.

All new resource IDs are minted through `lego/backend/internal/id`. The core records are:

| Record | Purpose |
| --- | --- |
| `workflows` | Mutable name and active-version pointer. |
| `workflow_versions` | Immutable source revision, image digest, SDK, and task manifest. |
| `workflow_runs` | Tenant-visible lifecycle, input reference, result reference, and Temporal execution IDs. |
| `task_runs` | Logical task calls and parent/child relationships. |
| `task_attempts` | Individual retry attempts, resource usage, and terminal outcome. |

API mutations write product state and an outbox record in one transaction. Idempotent dispatchers start or signal Temporal, avoiding an unsafe Postgres / Temporal dual write.

For each attempt, the trusted worker creates a short-lived `TaskAttempt` CR containing only identity, immutable image digest, plan, deadline, and retry number. The operator translates that CR into a Job and reports status. It does not read Postgres, call Temporal, interpret workflow policy, or carry tenant payloads. This preserves `operator → types ← backend`.

### 4. Task execution protocol

Every registration and task-execution Job contains:

- a trusted supervisor that exchanges payloads and status with bex-api; and
- the untrusted tenant image, which talks only to the supervisor through a Unix-domain socket.

The supervisor implements the Render SDK protocol needed by the public SDKs: input retrieval, task registration, callbacks, child-task submission, and child-result retrieval. The socket is an implementation detail; the stable contract is compatibility with the official TypeScript and Python SDKs.

Inputs and results move through authenticated supervisor calls or tenant-scoped object references. They are never placed in CR specs, Pod environment variables, command-line arguments, Temporal search attributes, or Kubernetes events.

### 5. Tenant isolation is a release gate

Workflow execution must not be enabled in multi-tenant production until all of the following controls are enforced. Namespace separation alone is insufficient, and ordinary `runc` containers are not the production trust boundary for hostile code.

| Layer | Required control |
| --- | --- |
| Compute | Registration and execution Jobs use a mandatory sandboxed `RuntimeClass` with a VM-grade boundary, such as Kata Containers or Firecracker. Plain `runc` is permitted only in explicit local-development mode. |
| Kubernetes | Each workspace executes in a dedicated namespace with restricted Pod Security, default-deny RBAC, ResourceQuota, LimitRange, and no automounted service-account token. Tenant code cannot create or inspect cluster resources. |
| Network | Default-deny ingress and egress. Tenant containers cannot reach Temporal, bex databases, the Kubernetes API, node or cloud metadata endpoints, or platform namespaces. DNS, same-workspace services, and explicitly configured public egress are separately allowed. |
| Credentials | Neither container receives a Kubernetes or Temporal credential. The supervisor receives a short-lived, single-attempt capability bound to workspace, run, attempt, allowed operations, and expiry. The tenant container receives no platform secret. |
| Local protocol | Every socket request is rebound to the supervisor's capability scope; tenant-supplied workspace, run, task, or attempt IDs are never trusted. A full tenant-container compromise can affect only its own attempt. |
| Data | OpenFGA checks every customer-facing operation. Postgres and object-store records are tenant-scoped. Payloads are excluded from Temporal, etcd, Pod specs, and platform logs. |
| Availability | Per-workspace limits bound queued runs, concurrent attempts, fan-out, recursion depth, input/result size, CPU, memory, PIDs, ephemeral storage, and wall time. Admission uses fair scheduling so one tenant cannot consume the global worker or cluster capacity. |
| Supply chain | Images are pinned by digest, scanned under the platform policy, and subject to the existing signing and admission controls. Registration uses the same isolation policy as execution because it also starts tenant code. |

The trusted supervisor is part of the isolation boundary and must remain small. It communicates with an internal, capability-scoped bex-api endpoint—not directly with Temporal or Postgres. Sidecar separation alone is not a security boundary; the design assumes the tenant container may compromise its own Pod, so the capability and upstream authorization must still constrain the blast radius to one attempt.

### 6. Execution semantics and versioning

Task execution is at least once. A lease timeout, lost completion response, or worker failure may repeat an attempt, so SDK documentation must require idempotent external side effects. Retry policy, timeout, and cancellation state are durable in Temporal and mirrored into the product records.

Workflow versions are immutable. New source or task registrations create a new version; existing runs remain pinned to the version and image digest with which they started. A run may wait for children without keeping a tenant container alive. In-process checkpoint and resume is deferred until its public semantics can be reproduced safely.

### 7. API compatibility

One backend core serves REST, GraphQL, MCP, and the dashboard, following ADR006. The minimum REST resource families are:

| Resource | Operations |
| --- | --- |
| Workflows | Create, list, get, update metadata, and delete. |
| Versions and tasks | Register a version, list versions, and inspect its task manifest. |
| Runs | Start, list, get, cancel, and retrieve result. |
| Run logs | Query stored logs and subscribe to live output. |

Render-compatible CLI and SDK behavior is verified at the boundary. Temporal workflow IDs, task queues, histories, and retry internals are not part of the public contract.

## Consequences

- Durable orchestration, timers, retries, and cancellation do not need to be reimplemented in bex.
- Shared Temporal keeps the control plane economical; the isolation boundary is the trusted worker plus sandboxed execution path, not a server per tenant.
- The design adds substantial operational surface: Temporal, sandboxed runtimes, fair admission, supervisor hardening, payload storage, and history retention.
- At-least-once execution is honest but requires customer guidance and idempotency tools.
- Workflows cannot ship safely as an API-only feature. Runtime isolation and adversarial verification are prerequisites.

## Alternatives considered

| Alternative | Decision |
| --- | --- |
| Implement a custom durable scheduler in Postgres | Rejected: reconstructs Temporal's history, timers, retries, and recovery semantics. |
| Run tenant functions inside Temporal workers | Rejected: tenant code could compromise trusted orchestration credentials and other tenants. |
| Deploy one Temporal cluster per tenant | Rejected as the default: high cost and operational overhead without removing the need to sandbox tenant code. Retained as a dedicated offering. |
| Use a Temporal namespace or task queue as the security boundary | Rejected: neither protects a credentialed or compromised worker by itself. |
| Run hostile code in ordinary containers | Rejected for production: namespaces and cgroups reduce accidents but do not provide the required kernel boundary. |
| Store payloads in Temporal or CRs | Rejected: increases control-plane data exposure and complicates retention and deletion. |

## Verification gates

Implementation may leave `Proposed` only after all gates pass:

1. The official Render TypeScript and Python SDK examples register tasks, start runs, invoke children, retry, cancel, and return results unchanged.
2. Unit and envtest coverage proves that `TaskAttempt` reconciliation creates digest-pinned, quota-bounded, restricted Jobs and rejects an unsandboxed RuntimeClass in production mode.
3. A live two-workspace test proves cross-workspace traffic and data access are denied while explicitly allowed same-workspace calls succeed.
4. A hostile task proves that Temporal, Postgres, the Kubernetes API, cloud and node metadata, platform namespaces, and another attempt's supervisor capability are unreachable.
5. Load tests prove per-workspace concurrency and fan-out limits, fair admission, cancellation, retry recovery, and bounded noisy-neighbor impact.
6. Temporal and bex-api restart tests prove that active runs resume without duplicate product records and that the outbox converges after partial failures.
7. The unmodified Render CLI workflow commands pass the compatibility checklist, and ADR018 records parity evidence for every exposed surface.
