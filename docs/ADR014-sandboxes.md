# ADR: E2B-compatible sandboxes — opensandbox pause/resume as hosted agent execution environments

**Status:** accepted gateway design. Pillar 5 was re-opened 2026-07-27 and create/list/get/exec/stop now run through a multi-node OpenSandbox **Kubernetes** runtime under gVisor. [ADR042-sandbox-cluster-substrate.md](ADR042-sandbox-cluster-substrate.md) is authoritative for the substrate, Render `ea sandbox` wire shapes, network/RBAC threat model, and rootfs-only pause limitation; E2B REST/gRPC compatibility remains deferred.

The original context and D2/D4–D6 discussion below are retained as design history. Where they describe a single-host Docker runtime, E2B routes, memory-preserving pause, or an unwired gateway, ADR042's implemented Kubernetes/Render contract supersedes them.

## Context

bex is an AI-native PaaS: agents are first-class users ([ADR008-vision.md](ADR008-vision.md)). Pillar 5 is **hosted execution environments for agents** — an agent spawns a sandbox, runs code in it, the sandbox auto-hibernates when idle ("sleep = free"), and wakes on next use. The compatibility play mirrors Render for the App API: speak **E2B's** shapes so existing agent tooling and habits transfer instead of restarting from zero.

The mechanism that makes "sleep = free" viable — sub-second wake — **already exists and is inherited**, it is not ours to invent:

- `lego/operator/internal/runtime/opensandbox.go` is a client for the OpenSandbox Lifecycle API (`BEX_OPENSANDBOX_URL`) exposing `Create` / `Pause` / `Resume` / `Delete` / `Endpoint`. `Pause`/`Resume` are **real snapshots** (~80 ms wake, [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md)). A per-sandbox **exec daemon** ships in the runtime (`execd_image = opensandbox/execd:v1.0.16`, `deploy/opensandbox/sandbox.toml`) — an exec channel is a real but currently unwired capability. The opensandbox controller also ships pre-warmed `pools` and `sandboxsnapshots`.
- This is the same class of mechanism E2B and Modal use to hit millisecond starts: **restore, don't boot**. E2B restores a Firecracker microVM memory snapshot (`MAP_PRIVATE` on-demand page faults + copy-on-write; template = Dockerfile→snapshot; ~5–30 ms resume, ~150 ms full restore). Modal uses gVisor `runsc` checkpoint/restore (CRIU) plus a lazy content-addressed image filesystem. bex gets this behavior **through opensandbox** rather than reimplementing it.
- Crucially, **E2B's `autoPause` + connect-resumes semantics are exactly this milestone's idle-hibernate** — so the work is orchestration, not new mechanism.

What is net-new is everything around that mechanism: a sandbox API distinct from the App CR, E2B-shaped surfaces, an idle tracker + wake-on-connect, an agent-facing exec surface, and an MCP spawn verb. Two constraints shape the design:

- **Architectural gap.** bex-api's feature services (`lego/backend/internal/*/service.go`, over the `internal/core` kernel) only patch App CR spec; they have **no opensandbox client** — the client lives in the operator binary. And that client's `imageEntrypoint` shells out to `docker inspect` (host-local), unusable from an in-cluster pod unless bypassed.
- **Runtime limit.** Real snapshot pause/resume works today only on the **Docker-runtime** opensandbox server (:8077); the k8s-runtime path is blocked on OrbStack (cri-dockerd, not containerd-CRI — [ADR001-go-and-gitops.md](ADR001-go-and-gitops.md)).

## Decision

Serve sandboxes as a **direct, synchronous gateway in bex-api over the opensandbox runtime**, E2B-compatible at the control plane, driven by agents over MCP. Eight decisions:

### D0 — Sandboxes are sessions, not services (the carve-out)

The App architecture rule (product action → **App CR contract** → operator converges; the operator exposes no API — [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md), [ADR003-control-plane.md](ADR003-control-plane.md)) governs **durable, declarative desired-state**. A sandbox is the opposite: an **interactive, ephemeral session** — created synchronously, exec'd against as a live stream, spawned and killed by the dozen. Forcing it through async CR reconcile fights its nature. Sandboxes are therefore **deliberately exempt** from the CR-contract rule; this ADR records that exemption so it reads as a decision, not drift.

|  | **App** (services) | **Sandbox** (sessions) |
| --- | --- | --- |
| shape | declarative desired-state | interactive, ephemeral |
| path | bex-api → **App CR** → operator converges | bex-api **gateway** → opensandbox (direct) |
| lifetime | long-lived, one per service | short-lived, many per agent |
| interaction | async (poll `status`) | synchronous (create/exec/connect now) |
| source of truth | App CR (+ future control plane) | opensandbox store (soft state in bex-api) |

### D1 — A direct synchronous sandbox service in bex-api; no CRD

Add a sandbox feature package in `lego/backend/internal/` that owns an **opensandbox client** (`BEX_OPENSANDBOX_URL`, the same endpoint the operator uses). bex-api becomes the **sandbox gateway**: control plane (create/pause/resume/kill), data-plane front (exec/connect proxy), auth (the existing bex-api credentials — API keys / OAuth2 or Kratos sessions, [ADR012-auth.md](ADR012-auth.md)), and activity observation, all in one place. No `Sandbox` CRD and no operator involvement — the operator keeps reconciling Apps only.

![Sandbox gateway: Claude Code or another agent calls the bex-api sandbox gateway over MCP (spawn_sandbox, sandbox_exec) and E2B tooling calls it over REST (/v1/sandboxes, /connect); the gateway drives the opensandbox runtime directly, control (create, pause, resume, kill) and data (exec via execd), and an idle sandbox pauses to the snapshot store and is restored on connect in about 80 ms](assets/ADR014-sandboxes.svg)

### D2 — Control-plane E2B compatibility now; spawn from templates

> **Amended 2026-11-12 (w2/m358, Render parity first).** A sandbox also starts from an image the workspace imported into its sandbox group (Render's `sandbox-groups images import` and `sandboxes create --image`). The image runs with the base template's entrypoint and environment, so the "image + entrypoint fixed at registration" rule below now holds for the entrypoint only. Design: [ADR104 § Images](ADR104-sandbox-render-parity.md#images-w2m358).

Serve E2B's REST **lifecycle** shapes so E2B lifecycle tooling transfers. Spawn from **pre-registered templates** (image + entrypoint fixed at registration), which takes the host-local `docker inspect` out of the hot path and matches E2B's own template model. Exec is surfaced through opensandbox `execd`. **Defer** full envd gRPC data-plane parity — `.pm/w2/m3/t002` already lists filesystem/proc APIs as out of scope, opensandbox already has its own exec daemon, and the primary consumer drives exec over MCP, not the E2B SDK's gRPC.

| E2B (control plane) | bex route | opensandbox call |
| --- | --- | --- |
| `POST /sandboxes` (templateID, timeout, **autoPause**, metadata, envVars) | `POST /v1/sandboxes` | `Create(template, env)` |
| `POST /sandboxes/{id}/connect` (resume if paused) | `POST /v1/sandboxes/{id}/connect` | `Resume` (if paused) → `Endpoint` |
| pause | `POST /v1/sandboxes/{id}/pause` | `Pause` (snapshot) |
| kill | `DELETE /v1/sandboxes/{id}` | `Delete` |
| commands (envd gRPC) — _deferred_ | MCP `sandbox_exec` / `run_code` | `execd` |

### D3 — Agents are external MCP drivers

Claude Code / any agent drives sandboxes **from outside** via MCP tools — `spawn_sandbox` and `sandbox_exec` (`run_code`) — following the shipped adapter pattern of the per-feature `mcp.go` fragments in `lego/backend/internal/` (streamable-HTTP at `/mcp` behind the auth gate; stdio via `api mcp-stdio`). The sandbox holds the agent's (untrusted) generated code; the agent stays outside. _(Deferred, not chosen: Claude Code running **inside** the sandbox as a hosted workspace — a different product that needs a per-sandbox external URL + gateway auth that does not exist yet.)_

#### D3a — Security boundary for external drivers

Every sandbox is mutually untrusted, including two sandboxes in the same workspace. gVisor is the syscall/kernel boundary; Cilium outside the guest removes same-namespace lateral traffic, filters DNS queries, and admits only platform-approved FQDN+TLS-SNI egress. There is no OpenSandbox iptables/nftables egress sidecar and no raw DNS exception. The exact source/destination/port/authentication matrix is normative in [ADR042 D4](ADR042-sandbox-cluster-substrate.md#d4--security-the-bex-api--opensandbox-link-is-a-single-trusted-hop).

The gateway persists reserved owner and workspace metadata and checks it after the ordinary workspace authorization on list/get/lifecycle/exec. An owner or explicit workspace administrator may operate the sandbox; every other caller receives the same not-found result as for an absent id. The lifecycle server and controller have cluster-wide informer-only RBAC (the controller cannot read Secrets cluster-wide); Kubernetes mutations come from RoleBindings scoped to provisioned `<ws>-sandbox` namespaces, and ingress to lifecycle TCP 8077 plus exec-gateway TCP 8081 selects the exact bex-api Pod and ServiceAccount identity rather than trusting its whole namespace.

#### Explicit workspace selection on GraphQL (w4/122)

The standalone sandbox GraphQL surface accepts `ownerId` consistently: `sandboxes(ownerId:)`, `sandbox(id:, ownerId:)`, and `terminateSandbox(id:, ownerId:)` select the same workspace as `createSandbox(ownerId:)`. Omitting it retains the caller's default-workspace behavior. REST uses `?ownerId`, and MCP list/stop tools use the shared `workspaceId` middleware; MCP has no separate by-ID sandbox read tool.

Resolvers only bind the requested workspace. The shared service still enforces workspace membership, sandbox ownership and the administrator override. A workspace selector does not grant access. The GraphQL by-ID query is classified as a read operation by the OAuth scope gate. The dashboard manages agent sessions through their own surface and has no standalone sandbox query to migrate; its generated schema still reflects these arguments. These are bex sandbox API semantics, not Render compatibility endpoints.

### D4 — Idle-hibernate = gateway-observed autoPause

The gateway tracks per-sandbox **last-activity** (every connect/exec resets it). A background sweeper pauses sandboxes past their idle window (opensandbox `Pause`); the next connect/exec on a paused sandbox **resumes first** (opensandbox `Resume`) — wake-on-connect. This reuses the `spec.idleTTLSeconds` semantics (declared but unread today, per [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md)) and maps 1:1 to E2B's `autoPause`. Idle state is **soft** — the sweeper's timers are rebuildable by listing opensandbox sandboxes on startup, since the opensandbox store is the source of truth.

#### Implemented: Render idle suspension (w2/m278, user decision D2=A 2026-11-07)

**Render's contract** (pinned OpenAPI `create-sandbox`, CLI v2.35 `--idle-timeout`, render.com/docs/sandboxes-creating and sandboxes-limits, checked 2026-11-07): `idleTimeoutSeconds` defaults to 900, `0` disables idle suspension, and a positive value is 300–86400. "A sandbox is considered idle if no command execution or file transfer is in progress." "Executing a command or transferring files restarts this timeout. These actions also automatically resume a suspended sandbox." "Background processes do not keep a sandbox active." Checking a sandbox's status does not resume it, a suspended sandbox has no compute or memory charges, and its "files, memory, and running processes are intact" after resume (Render runs each sandbox in a Firecracker microVM and suspends with a runtime snapshot by default). Render has no public pause/resume route: suspension is idle-driven and resumption is exec- or file-driven.

**Activity signal — bex-api's own brokered operations.** OpenSandbox exposes no activity signal, but every exec and file transfer goes through bex-api, so bex-api records them. `authorizeExecTarget` (legacy `/exec`, the pinned client's run-token mint and its stream redeem, MCP `sandbox_exec`) and `authorizeFileTarget` (file-token mint and transfer redeem) touch `sandbox_activity.last_activity_at` (migration 0208). An open exec stream or file transfer re-touches it every minute and once more when it ends, so a command in progress is never idle and the window runs from its end, as on Render. Status reads (`GET`, list, GraphQL, MCP `list_sandboxes`) never touch it. The record lives in Postgres, not in memory, so every replica sees every replica's activity; it is still soft state, rebuilt from the create time for a sandbox with no row.

**Suspend mechanism — OpenSandbox pause.** The `IdleSuspender` runs on every bex-api replica every 30 s. For each workspace's tenant key it lists sandboxes, and for each running ordinary sandbox whose stamped `bex.co/idle-timeout-seconds` is positive and whose last activity (or create time) is older than it, it takes a compare-and-set claim (`suspend_claimed_at`) and calls OpenSandbox `POST /sandboxes/{id}/pause`. Only one replica wins a claim. The status reads `suspending` (OpenSandbox `Pausing`) while the rootfs commit runs, then `suspended` (`Paused`). The sweeper records the `suspending` meter phase at the moment it pauses, so `sandbox_compute_seconds` stops there, not at the next 30 s meter poll. A failed pause releases the claim. Scale-to-zero with a PVC and snapshot+delete+restore were considered. The first would need the sandbox's writable layer on a volume, which neither the BatchSandbox template nor gVisor's rootfs gives today. The second is what OpenSandbox pause already does internally (commit to the per-tenant registry, recreate from the snapshot image, ADR042 D5), so building it again in bex-api would add nothing.

**Wake on exec.** After authorization and before any ticket is minted, `ensureRunning` resumes a `suspended` sandbox (`POST /resume`, a 409 meaning another request is already resuming it), applies the same billing gate as the explicit resume verb, and polls until OpenSandbox reports `Running`. It also waits out `suspending`/`resuming`. The race in which an exec reads a sandbox as running just after a sweeper claimed it is closed by the claim: the touch returns the claim, and the exec waits up to 30 s for that pause to land and then resumes, rather than running into it. A claim older than 30 s belongs to a sweeper that died before pausing and is cleared. The wait is bounded. The REST exec and file routes extend their write deadline and wait 3 min (a cold resume pulls the snapshot image and schedules a pod). MCP `sandbox_exec` waits 20 s, inside the 30 s HTTP write timeout. A slower resume answers `503 SANDBOX_RESUMING` naming a retry; a resume that lands in `Failed` answers `503 SANDBOX_RESUME_FAILED`. The pinned CLI's mint request has no client timeout, so `bex sandboxes exec` and `copy` on a suspended sandbox block and then run.

**What survives.** Files and installed tools on the root filesystem survive, and so do the id, plan, network policy, timeout and idle timeout. Memory, running processes and `/tmp` do not (ADR042 D5: rootfs commit, and gVisor's internal `/tmp` tmpfs). This is weaker than Render's runtime snapshot and is recorded in ADR018 § Sandbox create flags as a divergence awaiting sign-off. The k8s path to parity is a memory checkpoint (`runsc checkpoint` / containerd CRIU), the ADR042 D5 watch item.

**Runtime timeout across suspension (w2/m278 t010).** Render restarts the runtime timeout when a suspended sandbox resumes, and a suspended sandbox stays resumable for 30 days (render.com/docs/sandboxes-limits: "If a sandbox is suspended and later resumed, this timeout restarts", checked 2026-11-07). bex matches this:

- **Stamps.** Every pause of an ordinary running sandbox, from the idle sweeper or the explicit verb, first stamps `bex.co/suspended-at` (Unix seconds) on its OpenSandbox metadata (`PATCH /sandboxes/{id}/metadata`). Every resume of a suspended one, from wake-on-exec or the explicit verb, first stamps `bex.co/resumed-at`. The stamp is the transition's precondition. If it fails, the sweeper releases its claim and leaves the sandbox running, and a resume is refused. A pause or resume that the sandbox's phase would refuse writes no stamp.
- **Deadline** (`lifetimeDeadline`, read by the inventory reconcile from the list it already makes). A `suspending` or `suspended` sandbox with a suspension stamp is reaped 30 days after it, whatever its timeout, including no timeout. Its timeout does not run while it is suspended. Otherwise the timeout runs from the later of `createdAt` and the last resume. An unstamped paused sandbox keeps the creation rule: one paused before t010, or an agent-session sandbox.
- **OpenSandbox expiry.** The BatchSandbox controller (pinned commit `e95681e`, `batchsandbox_controller.go`) deletes a BatchSandbox once `spec.expireTime` passes, whatever its pause state. Without a renewal, a paused sandbox would still be deleted at `createdAt + timeout`. opensandbox-server 0.2.2 exposes `POST /sandboxes/{id}/renew-expiration` with `{"expiresAt": RFC 3339}`. Any future time is accepted, earlier or later than the current one. It answers 409 for a sandbox created without an expiry, and an expiry cannot be cleared. So before pausing a timed sandbox, bex-api moves its expiry to the pause + 30 days, and a failed renewal refuses the pause. After a resume is accepted, it moves the expiry to the resume + timeout (raised to the 60 s floor), and a failure there is logged, since the reconcile still enforces the exact deadline. A no-timeout sandbox has no expiry to move. The reconcile reaps it 30 days into a suspension.
- **Inventory class.** A sandbox whose deadline falls more than 24 h after its creation counts as `long_lived`, which `SandboxOldestOverBound` excludes. This covers a suspended sandbox and one resumed late in its life, as well as the long timeouts below.

**Agent-session sandboxes are excluded.** They carry no idle stamp (they report `0`), the sweeper skips them, and generic exec neither tracks their activity nor resumes them. Their lifecycle belongs to the agent-sessions feature (turn dispatch, Completer idle-grace teardown, ADR047), and an unrequested pause would kill the running driver, which is a process.

**Store-off deployments** have no activity record and no tenant-key list, so they do not idle-suspend; a sandbox paused by other means is still resumed by the next exec. Sandboxes created before w2/m278 carry no stamp, keep reporting `idleTimeoutSeconds: 0`, and never idle-suspend.

### D5 — Cold-start/wake is inherited, not invented

bex **orchestrates**, it does not build, fast start: opensandbox **snapshots** give wake latency, pre-warmed **pools** give create latency (the two levers E2B/Modal use). Honest bounds: **wake latency = opensandbox checkpoint/restore (~80 ms), available today only on the Docker-runtime server (:8077)**; the k8s-runtime snapshot path is blocked on OrbStack. First-create latency is hidden by pool pre-warm when configured. bex's contribution is the _policy_ (when to pause/resume), not the _speed_.

### D6 — Capacity & admission control (two bounds, not one pool)

"Pooling" is only half the story, and conflating the two halves miscounts resources. There are **two distinct mechanisms**: a **pre-warm pool** (a free-list of ready sandboxes/snapshots that hides _create_ latency — classic pooling, sized `min_ready`/`max_ready`, refilled async) and **idle-hibernate overcommit** (which is _not_ a pool but demand-paging: idle sandboxes are paused and later resumed on use). They bound different resources, because a paused sandbox **frees RAM/CPU but still costs disk** (its memory+rootfs snapshot):

| bound | limited by | who enforces |
| --- | --- | --- |
| **concurrent running** (unpaused) | RAM / CPU (hard) | gateway admission + LRU evict-to-pause |
| **total existing** (incl. paused) | disk (snapshots) + per-tenant quota | gateway quota check |
| **warm-ready** | `min_ready` / `max_ready` | pool refiller |

Because bex-api is the single funnel (D1), it is the natural **admission-control** point. The knobs:

- **Concurrent-running cap → LRU evict-to-pause.** When admitting a create/resume would exceed node capacity, proactively `Pause` the least-recently-used _running_ sandbox to make room (evict before its idle window). This is the overcommit: running-set ≤ capacity, existing-set ≫ capacity.
- **Per-tenant quota by tier.** Mirror E2B/Modal plan limits (max concurrent sandboxes + max timeout) by reusing the existing `Tier` ladder that already maps to resources for Apps.
- **Warm-pool bounds.** `min_ready`/`max_ready`; keep the pool as _paused_ snapshots so it doesn't burn RAM while idle-warm (opensandbox `pools`).
- **Node-level elasticity (deferred).** Unschedulable sandboxes → pressure → autoscaler adds a machine; empty nodes → scale down — the same bin-pack + idle-evict loop Apps use ([ADR002-architecture.md](ADR002-architecture.md), "node-aware but provision-unaware"). **This does not hold yet:** the Docker-runtime opensandbox is **single-host** (:8077), so today's capacity control is the single-host subset (concurrent cap + evict + quota); cross-node scheduling waits on the k8s-runtime snapshot path (containerd-CRI cluster).

### Lifetime bound (w5/m99, amended by w2/m263 D3=A and w3/m303)

A sandbox's lifetime follows Render's `timeoutSeconds` contract. On 2026-11-07 the user chose to match Render (w2/m263 D3=A), which then capped Hobby at 24 h. Render removed that cap on 2026-11-03 (CLI commit `c280b0b6`, INF-7261 "remove the 24-hour sandbox lifetime limit for Hobby", shipped in CLI v2.36.0; [render.com/docs/sandboxes-limits](https://render.com/docs/sandboxes-limits) lists one limit for every workspace), so w3/m303 follows it. The contract is now the same on every plan:

| Request                               | Every workspace               |
| ------------------------------------- | ----------------------------- |
| `timeoutSeconds` omitted              | 86400 s (24 h)                |
| `timeoutSeconds: 0`                   | no timeout; reported as `0`   |
| `60` … `2592000` (30 days)            | honored                       |
| `1` … `59`, below 0, or above 2592000 | `400 SANDBOX_TIMEOUT_INVALID` |

The 60 s minimum is the pinned `CreateSandbox` description ("Positive values must be at least 60"); before w3/m303 bex accepted 1–59 and enforced them at the exact deadline. Agent-session sandboxes keep the 86400 s bound on every plan.

bex-api stamps the effective lifetime into `bex.co/timeout-seconds` metadata. The value is seconds, or `none` for no timeout. A distinct marker is used because legacy pre-w5/m99 metadata `0` still means 86400 s, so no old sandbox becomes immortal. REST, GraphQL and MCP return the effective value, and `0` means no timeout, as on Render. OpenSandbox gets no `timeout` for a no-timeout sandbox, so the BatchSandbox has no `expireTime`. A positive value goes to OpenSandbox raised to its 60 s floor. Its `max_sandbox_timeout_seconds` is 2592000, a backstop under bex-api's per-plan check.

The Completer inventory reconcile terminates sandboxes past their timeout, including a sub-minute timeout at its exact deadline. The timeout runs from `createdAt`, or from the last resume (Render restarts it on resume). A suspended sandbox is kept for 30 days from its suspension instead ([D4](#d4--idle-hibernate--gateway-observed-autopause), t010). A running no-timeout sandbox is never terminated. An ordinary sandbox whose deadline falls more than 24 h after its creation counts in the `long_lived` inventory class, which `SandboxOldestOverBound` excludes. Every other class keeps the 24 h alert. For agent-session sandboxes, session-side teardown (cancel, idle grace, previous-sandbox intents) is the fast path, and inventory reconcile is the backstop when a row forgot its `sandbox_id`.

The orphan-cost risk w5/m99 guarded against (an orphan that ran 403 h and cost $88.50) is now carried the way Render carries it. An owner who asks for no timeout is billed for the time the sandbox runs. Idle suspension (w2/m278, D4) stops a forgotten sandbox from running unattended: after 15 minutes without an exec or file transfer it is suspended and stops billing compute.

### D7 — State model & durability (a ladder, mostly deferred above pause/resume)

"User state" is not one thing — it is a ladder of state with different survival boundaries. bex is honest about which rung is strong today and which are promissory:

| state | survives | mechanism | today |
| --- | --- | --- | --- |
| fs (inside sandbox, except `/tmp`) | **pause/resume** (idle suspension, D4) | opensandbox rootfs snapshot | rootfs only on the k8s/gVisor substrate (ADR042 D5): memory and processes do not survive |
| sandbox rootfs | **not `kill`** | — | `kill` is destructive by design |
| data meant to outlive a sandbox | across `kill` / new sandbox | persistent **volume** (opensandbox `[storage]`) | deferred (t004 "persistent sandbox volumes") |
| owner → sandbox (reconnect) | across sessions | sandbox id + reserved owner/workspace metadata | implemented; owner or explicit workspace admin only |
| the snapshot store itself | **host loss** | single-host sqlite + disk | no HA |

The stance: **a sandbox is an ephemeral session** (D0). Its in-sandbox state is faithfully preserved **across pause/resume** and **deliberately destroyed on `kill`** — that is the contract, not a bug. Everything more durable is an explicit, deferred layer:

- **Persistence beyond a sandbox → volumes.** To keep data across `kill` or share it across sandboxes, mount an opensandbox volume (bex's counterpart to E2B `volumeMounts`); the rootfs is never the durable store. **Deferred**, aligned with t004's out-of-scope.
- **Per-user ownership keys on the resolved caller identity.** Reconnect works off the sandbox id plus reserved owner, workspace, sandbox-regime, and effective-policy metadata. The gateway stamps these values on `Create`; list/get/lifecycle/exec hide other members' objects and incompletely hardened resources behind the same not-found response. Only an explicit workspace administrator may override the owner check. Caller metadata is not accepted, and legacy ownerless objects fail closed. Exec calls this same centralized resource check before minting the ssh-gateway ticket.
- **No durability beyond the host.** All sandboxes and snapshots live on the single opensandbox host's sqlite + disk — host loss loses them, the same no-HA gap [ADR002-architecture.md](ADR002-architecture.md) flags for App state in single-node etcd. HA/replication waits on a multi-node runtime.

## Alternatives considered

- **`Sandbox` CRD reconciled by the operator** (mirror the App contract) — rejected. Async reconcile fights synchronous create/exec/connect (callers want a handle _now_, not a `status` to poll); the exec data plane still needs a synchronous gateway, so it is never "pure CRD"; and an etcd write + reconcile per throwaway box is heavy for high-churn ephemeral sessions. The App contract is right for durable services (D0), wrong for sessions.
- **Full envd (E2B data-plane) parity** — rejected for now. It reimplements E2B's gRPC data plane and access-token model while opensandbox already ships `execd`, and the payoff is low when the primary consumer drives exec over MCP. Revisit if first-class E2B-SDK `commands.run` / filesystem support becomes a requirement.
- **Claude Code inside the sandbox** (hosted-workspace product) — deferred, not rejected. Compelling ("hosted Claude Code, sleep = free") but a different product: it needs per-sandbox external URLs, a gateway with per-sandbox auth, and an attach/terminal surface — none of which exist. Recorded as future once the edge/gateway lands.

### How other sandboxes store state (prior art for D7)

The industry splits into two camps for what survives an idle-sleep, plus a shared "volumes for durable data" layer. bex's choice (D5/D7) is deliberate, not default:

| provider | idle-sleep preserves | durable across destroy/`kill` | model |
| --- | --- | --- | --- |
| **E2B** | filesystem **+ memory** (processes, variables) | volumes (beta) | pause/resume snapshot |
| **Fly.io Machines** | filesystem **+ memory** (suspend) | attached volumes | suspend/resume snapshot |
| **Modal** | memory snapshot → restores as a **new** sandbox | volumes | snapshot-restore (not same instance) |
| **Cloudflare Sandbox** | **nothing** — `sleepAfter` (~10m) stops the container; next request boots fresh, state reset | Durable Object (SQLite) storage + mounted **R2** buckets — persisted by _your_ code | ephemeral compute + external store, coordinated by a Durable Object |
| **Vercel Sandbox** | filesystem + packages (**no** memory/processes) | rebuild from snapshot | filesystem-only snapshot |
| **Daytona** | filesystem (stop/archive) | persistent volumes | container stop + volumes |
| **Blaxel / Sprites** | state (hibernated; bills nothing while idle) | volumes | hibernate-first |
| **bex (this ADR)** | filesystem **+ memory** (opensandbox snapshot) | volumes (deferred) | pause/resume snapshot |

Three lessons shape D7. **First, the split is real:** the snapshot-resume camp (E2B, Fly, and the hibernate-first Blaxel/Sprites) keeps full memory+process state across sleep and resumes the _same_ instance; the ephemeral-compute camp (Cloudflare, Vercel) throws the box away on sleep and makes you persist state to an external store yourself. bex sits in the **snapshot-resume camp** — opensandbox already gives memory+fs snapshots (D5), so "sleep = free" preserves the running session transparently rather than forcing the agent to externalize state. **Second, Cloudflare is the sharpest contrast worth calling out:** its sandbox is a container fronted by a **Durable Object**; the container filesystem is fully ephemeral (lost when `sleepAfter` stops it), so durable state lives _outside_ the sandbox — in the Durable Object's SQLite storage (config/results) or in **R2 buckets mounted as filesystem paths** that survive destroy-and-recreate. That is a valid design, but it pushes state management onto the app; bex instead inherits transparent memory+fs snapshots so an idle agent session just resumes. **Third, everyone converges on volumes** as the survive-destroy layer — no provider makes the sandbox rootfs durable; data meant to outlive a box goes on a mounted volume/bucket (Cloudflare's R2 mounts, E2B/Modal/Daytona/Fly volumes). That is exactly bex's D7 stance (rootfs is ephemeral; durability = opensandbox volumes, deferred), so the plan is aligned with the field, not novel.

## Consequences

- bex-api gains a responsibility beyond CR-patching (it talks to a runtime directly) — a **documented exception** to the operator-is-the-only-mechanism rule, scoped strictly to sandboxes.
- bex-api needs **opensandbox reachability** (`BEX_OPENSANDBOX_URL`) and a **template registry**; the current host-local `imageEntrypoint`/`docker inspect` path is bypassed by templates.
- **Idle state is soft** (in-memory timers, rebuildable from opensandbox) — acceptable because opensandbox persists sandbox existence/state.
- **Wake latency is runtime-dependent**: Docker-runtime first; k8s-runtime snapshots wait on a containerd-CRI cluster.
- **No per-sandbox public URL yet** — agents reach exec through the bex gateway, not a stable external sandbox URL; that (and the inside-the-sandbox product) waits for the edge/gateway.
- **Capacity control is single-host until multi-node lands** (D6): concurrent-running cap + LRU evict-to-pause + per-tenant quota work today on the one Docker-runtime host; cross-node scheduling and autoscaler-driven elasticity for sandboxes wait on the k8s-runtime snapshot path.
- **Per-user state keys on the resolved caller identity** (D7): sandbox ownership is stamped with the OAuth2/Kratos caller (`api.IdentityFrom`), not a shared token; full multi-tenant isolation matures with the control plane, and durable-across-`kill` data still needs volumes (deferred).
- **No durability beyond the host** (D7): sandboxes + snapshots live on one opensandbox host with no HA — the same single-node-state gap as App etcd; host loss loses them until a multi-node runtime.
- **Implementation targets** (m3, not this ADR): template-aware `Create` + `Exec` over `execd` in `lego/operator/internal/runtime/opensandbox.go`; a sandbox feature package in `lego/backend/internal/` (service + E2B REST adapter in its `rest.go` + `spawn_sandbox` / `sandbox_exec` tools in its `mcp.go`); an opensandbox client wired into bex-api in `lego/backend/cmd/api/main.go`. The `spawn_sandbox`/`sandbox_exec` MCP tools belong on the existing bex-api MCP server (per-feature `mcp.go` fragments), as `.pm/w2/m3/t004` now records.

## Future considerations

### Deliverables — externalize outputs; the rootfs is not the deliverable

A sandbox is the agent's live **workbench**, not the user's disk. Session state (memory, processes, filesystem) is preserved transparently across pause/resume (D5/D7), but anything a product must _keep_ — a report, generated files, a build artifact, a code change — has to leave the sandbox, because the rootfs is destroyed on `kill` (D7) and is single-host with no HA today. Three externalization paths, in rough order of preference for agent-UI products (Manus / GitHub-coding-agent style):

- **Push out (git / PR)** — the coding-agent shape: the deliverable is a commit/PR, the agent `git push`es it, and the sandbox stays fully ephemeral. Cleanest — no durable sandbox state needed.
- **Object storage / artifact upload** — the Manus shape: the agent uploads results (files, reports) to a bucket at end-of-task; the sandbox is still throwaway.
- **Persistent volume** — for data that must survive across sandboxes or long-lived stateful workspaces; mount an opensandbox volume (D7). Deferred (t004), and the heaviest option — reach for it only when push-out / upload don't fit.

The product contract to document for consumers: **treat the sandbox as an ephemeral live workspace; persist deliverables out-of-band.** This is what keeps "sleep = free" and single-host-no-HA acceptable — a lost host loses only live _intermediate_ state, never a committed deliverable.

### Surfacing work to a human UI

Driving these sandboxes from a product UI — an agent-task dashboard (Manus-style) or metered "premium request" runs — needs a **browser-facing live surface**: streamed terminal / step / diff output over WebSocket/SSE from the gateway. Today's agent-facing surfaces (MCP `sandbox_exec` + the per-sandbox HTTP endpoint) do not yet provide that. The natural home is the dashboard (`dashboard/`, already a bex-api GraphQL client). Per-run / "premium request" metering reuses the quota hook (D6) and the metrics surface.

### Repo-backed sessions (coding-agent shape)

A coding agent working a git repo (the "premium request → PR" pattern) is the textbook case of the deliverables split: the **working tree + installed deps + build cache are session state** (snapshotted across pause/resume), and the **commit / PR is the deliverable** (pushed out). So sandbox ephemerality and single-host-no-HA don't bite — the PR already lives on the git host. Two shapes, both supported:

- **One-shot (premium request → PR)** — fresh sandbox per task: clone @ ref, work, open PR, discard. Ephemeral is fine; matches GitHub's coding-agent model.
- **Iterative session** — the agent works the same repo over many rounds; snapshot-resume keeps deps / build-cache / half-run tests warm across idle, so each round is cheap. This is bex's edge over the ephemeral-compute camp (Cloudflare / Vercel), which re-clone and re-install on every wake.

Reuse and unification:

- **Templates are the devcontainer equivalent** — bake the repo's toolchain (optionally repo + deps) into a template so spawn is instant (D2); bex's existing build-from-git plumbing (`internal/build`, buildpack detection) can generate it.
- **The unified loop** — agent edits the repo in a sandbox → PR → merge → the same opensandbox runtime deploys that repo as an App. "Edit code → deploy" on one platform, one auth, one `Core` — a story a standalone sandbox product can't tell.

Security decisions this raises (to settle when the milestone lands):

- **Git credentials stay out of the sandbox.** A repo token / deploy key must never enter the sandbox — untrusted repo code could exfiltrate it. Prefer a **git-credential proxy at the gateway**: the sandbox's git talks through the gateway, which injects a short-lived scoped token; the secret lives only gateway-side. Stronger than passing it as an `envVar`.
- **Egress allowlist.** Running a repo's tests runs arbitrary repo code with network access; allowlist the git host + package registries and deny the rest (opensandbox `[egress]`), or exfiltration / SSRF is wide open.
- **Isolation.** Untrusted repo code is the strongest case for microVM isolation (Kata / Firecracker on bare-metal) over hardened containers.

## Verification (when m3 lands)

Against a local Docker-runtime opensandbox (`bash scripts/up.sh` / `scripts/start-opensandbox.sh`, :8077) — the DoD loop from `.pm/w2/m3/README.md`:

1. MCP `spawn_sandbox` returns a sandbox id + reachable endpoint.
2. `sandbox_exec` runs a command and returns its stdout.
3. Left idle past the window, the sandbox auto-pauses (opensandbox reports paused; it occupies nothing).
4. The next `sandbox_exec` transparently resumes it (wake-on-connect) and succeeds.
5. **State survives multiple cycles** — write a file and set an in-process variable, then pause→resume **twice**; both persist after the second resume (guards against the E2B-style bug where fs changes were lost on the second resume).
