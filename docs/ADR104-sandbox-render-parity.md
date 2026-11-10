# ADR104 — Sandbox Render parity: the contract bex matches, and every gap with an owner

**Status:** proposed (2026-11-08; refreshed against the repo, and D8 decided, 2026-11-10) · **Extends:** [ADR008 § Render parity first](ADR008-vision.md#render-parity-first), [ADR018](ADR018-render-parity.md) (§ Sandbox create flags), [ADR042](ADR042-sandbox-cluster-substrate.md) (substrate and network policies), [ADR014](ADR014-sandboxes.md) · **Amends when accepted:** ADR018 (sandbox rows), ADR023, ADR030, [cli-compatibility-checklist.md](cli-compatibility-checklist.md) · **Board:** w2/m278 (suspend, blocked), w32/m283 (network policy, done), w32/m284 (SDK walk, blocked), w32/m285 (snapshots, blocked), w32/m297–m299 (env, execution history, SDK gaps; done)

## Context

Render Sandboxes entered **public beta on 2026-11-04**. Until now bex tracked them piecemeal: ADR042 chose the substrate and the `ea sandbox` wire shapes, ADR018 graded individual create flags, and three milestones (m278, m283, m285) each re-read one slice of Render's docs. No single document says what "sandbox parity" is, which rows are green, and which gaps are open. This ADR is that document.

It is built from a sourced research pass on 2026-11-08 (fan-out search, 19 sources fetched, 25 claims put to a three-vote check by model reviewers, 23 confirmed, 2 refuted). **Every confirmed claim comes from render.com's own pages** (`/sandboxes`, `/docs/sandboxes*`, the TypeScript and Python SDK docs); none was corroborated independently, and the product was four days old when researched, so each limit below can change. Re-check the sources in § Sources when you act on a row.

**Refreshed 2026-11-10.** The repo moved after the first draft. w32/m283 (network policy), m297 (`env`), m298 (execution history) and m299 (SDK path and mint gaps) are done and verified on production, and w5/m287 serves sandbox `name`. ADR018's sandbox rows moved, so this ADR cites them by name, not by line number. m278 (suspend), m284 (SDK walk) and m285 (snapshots) are **blocked**: m278 on the D5 sign-off and a production walk, m284 on QA credentials, m285 on m278. The first draft was numbered ADR102; it is ADR104 because another ADR102 (agent child sessions) landed first and w3/m304 t003 claimed ADR103 at filing.

**Not established by the research, and therefore not decided here:** the exact `/v1/sandboxes` and `/sandbox-groups` REST schemas for groups, images and snapshots; the `render sandboxes` and `sandbox-groups` flag set beyond what the pinned CLI v2.35 already exercises (the repo's own pin and OpenAPI copy are the authority for those); and **pricing, including whether a suspended sandbox bills and what snapshots cost**. The research's two pricing claims were each rejected by all three reviewers, which is a gap in evidence, not evidence of the opposite. Follow-up reads of Render's launch post (2026-11-08) and its comparison article (2026-11-06) answer both the billing model and the rates; see D8, which decides to adopt them. Remaining unknowns are tracked as open questions.

## Implementation at a glance

![Sandbox architecture and Render-parity state](ADR104-sandbox-render-parity.svg)

Every client reaches bex-api on Render's `/v1/sandboxes*` paths. bex-api authorizes, tracks idle time, mints the exec and file connect tokens, and is the **only** caller of the OpenSandbox lifecycle API; the ssh-gateway is the only holder of `pods/exec`. Sandboxes run as gVisor pods in a per-tenant namespace, and Cilium decides what they can reach. Solid green is shipped and verified live, dashed green is shipped with the live check pending, yellow is scheduled, orange diverges (memory and processes on suspend, D5; Docker in a sandbox, D6). Source of the picture: ADR042's architecture and network-policy sections, plus ADR018's sandbox rows.

## Decision

### D1 — The parity target is Render's documented contract, graded row by row

The rows below are the **contract**: what Render does and what bex must do. A row is **✅** when shipped **and** verified live, **🚧** when shipped but not yet verified live, or scheduled on a named milestone, **⚠️** when it diverges. A divergence stands only if ADR018 records it with the user's sign-off (AGENTS.md rule); a ⚠️ here without that is a defect to close, not a decision.

**Authority.** The state ledgers are [ADR018](ADR018-render-parity.md) (§ Sandbox create flags and the sandbox rows) and [cli-compatibility-checklist.md](cli-compatibility-checklist.md); this ADR owns the contract and the decisions. The State columns below are a dated snapshot (2026-11-10), not a second ledger. Landing this ADR requires amending the ADR018 rows it contradicts (gap 11), so the two cannot disagree.

**Surfaces.** Every row applies to REST, GraphQL and MCP (the dashboard is N/A for sandboxes except groups: Render's docs show a Dashboard "+ New > Sandbox Group" menu, with an optional project environment, read 2026-11-10; w22/m311 t012). A row is not ✅ until all three answer the same way.

### D2 — Lifecycle, timeouts and suspension

| Render behavior (render.com/docs/sandboxes-limits, SDK docs) | bex | State |
| --- | --- | --- |
| Statuses `creating`, `running`, `suspending`, `suspended`, `resuming`, `errored`, `terminated` | Every status occurs in code since w2/m278; not yet seen live | 🚧 m278 t009 |
| Runtime timeout: default 86400 s; `0` disables; otherwise 60–2592000 s | Shipped (w2/m263 D3=A) | ✅ |
| Runtime timeout restarts on resume and restore | Shipped for resume (w2/m278 t010), live check pending; restore is m285 | 🚧 m278 t009, m285 |
| Hobby plan timeout cap | bex caps Hobby at 86400 s and holds `0` to it (ADR018, w2/m263 D3). Render's CLI v2.36.0 (2026-11-06) **removed** the Hobby 24 h wording from `--timeout` help, so Render may have lifted it. Not recorded as a signed divergence | ⚠️ gap 9 |
| Idle timeout: default 900 s; `0` disables; otherwise 300–86400 s; idle sandboxes suspend automatically | Shipped in code (w2/m263 D2=A, w2/m278); live check pending | 🚧 m278 t009 |
| A suspended sandbox stays resumable for 30 days, subject to group capacity; exec, upload and download each resume it | Shipped in code (w2/m278 t010); live check pending | 🚧 m278 t009 |
| Terminate is idempotent and permanently deletes the filesystem | Shipped | ✅ |
| Exec runs via `bash -c`, streams `output` events then `exit`; a nonzero exit code is data, not an error; list is newest first, excludes `terminated` unless filtered in, and caps `limit` at 100 | Exec, SSE and the status filter shipped and live-verified (m147, m263). The `bash -c` form and the list `limit` cap are not pinned by a test | 🚧 gap 4 |
| Execution history: `GET /sandboxes/{id}/execs` | Shipped (w32/m298), verified on production 2026-11-10 | ✅ |
| Suspend keeps files, memory and running processes (runtime snapshot by default) | Files only: the rootfs is committed and the pod recreated, so memory and processes are lost and `/tmp` is empty (ADR042 D5) | ⚠️ D5 |
| Create inputs `env`, `name`, `group` | `env` is served (w32/m297: 100 variables, 256 KB, write-only) and so is `name` (w5/m287). `group` is served by w22/m311: a sandbox is created in the named group or the workspace's default one, which is created on first use; the code is in, the production walk is w22/m311 t007 | ✅ `env`, `name`; 🚧 `group`, m311 t007 |
| Billing: running sandboxes bill used CPU plus allocated memory; suspended bill snapshot storage only | Different model today; D8 decides to adopt Render's | 🚧 D8 |

### D3 — Network policy

Matches Render and is shipped by w32/m283: `allow-all` is the default, `deny-all` reaches nothing, and `allow-list` takes 1–100 `{domain, protocol: "https"}` rules, lowercase, no scheme, port or path, with `*.example.com` covering subdomains but not the apex and an exact name not covering subdomains. Rules must be omitted for the other two types. Traffic matching no rule is blocked. The platform floor (nodes, Kubernetes API, metadata, private ranges, other tenants) stays closed under every policy; that is bex's isolation invariant, not a Render row, and Render does not document it as customer-visible.

One rule is easy to miss: **a restored sandbox inherits the source's policy, including allow-list rules, and the policy can be overridden at restore.** `allow-all` as the default applies to a plain create only. m285 must implement inheritance; m283 does not cover it. These rows are ✅: w32/m283 is done and enforcement was verified on production (2026-11-08; ADR018's `networkPolicy` row is ✅). The inheritance rule above is still open.

### D4 — Snapshots

m285 is the vehicle. It is blocked behind w2/m278 (its production walk and the D5 decision), so gap 2 and the D5 sign-off sit on its critical path. The contract it must match:

- **Kinds:** `runtime` (default; filesystem plus memory and CPU, restorable only on the identical plan, else `SnapshotPlanMismatchError` in the Python SDK) and `filesystem` (writable filesystem only, restorable on any plan). The source keeps running during capture.
- **Statuses:** `creating`, `available`, `failed`; only `available` restores; deleting a `creating` snapshot is refused; `failed` carries an `error` field.
- **Expiry and names:** 30 days from the capture request by default; a custom `expiresAt` must be in the future (SDK-documented only); restore does not extend it; an expired snapshot cannot be fetched or restored and is omitted from lists, but sandboxes already restored from it are unaffected. Names are 1–64 characters, case-sensitive, scoped to the group, reusable, must not begin `snp-`, and resolve to the most recent `available` snapshot of that name.
- **Restore:** creates a new sandbox and keeps the snapshot; takes `snapshotId` or `snapshotName`, never both; works only inside the snapshot's own group; the filesystem is always preserved; processes survive only for runtime snapshots, keep their original environment, and environment variables are not inherited; plan, network policy, runtime timeout and idle timeout default to the source's values, including a disabled timeout, and can be overridden.

**Security of the capture path.** Today's suspend path runs a commit Job in `opensandbox-snapshot`, a PSS-privileged namespace that mounts the node's `containerd.sock` (ADR042 D5), which is node-root equivalent. m285 lets a user trigger it on demand, so m285 t001 must fix, before t002 ships: that the Job spec contains nothing a caller chooses (image, mounts, arguments), a per-sandbox and per-workspace rate limit, a size and time bound on a capture, and that a failed capture cannot leave a privileged Pod behind. This makes a privileged path user-triggerable, so route it through the security review lineage.

`runtime` is Render's default kind and bex cannot capture memory today. That is the same gap as D5 and is resolved with it: until a memory checkpoint exists, `--kind runtime` answers a named 400, and ADR018 carries it as a divergence awaiting sign-off (as m285 t001 already plans).

### D5 — The main substrate divergence: memory and processes

Render's microVM snapshots memory, so suspend and runtime snapshots keep running processes. bex's gVisor/OpenSandbox substrate commits the root filesystem only. This is the largest divergence in what a customer's code observes (the others are D6 and the Hobby cap), and it appears twice (suspend, runtime snapshot). It is already recorded in ADR018 as awaiting sign-off; this ADR does not settle it. It lists the two honest ways to close it so the decision is not an open-ended wait:

1. **Build the checkpoint** (`runsc checkpoint`/CRIU under gVisor; the ADR042 D5 watch item). Full parity; large, with an unproven restore path.
2. **Accept filesystem-only** and document `--idle-timeout 0` as the workaround, with the divergence signed off in ADR018.

Per the parity rule, the board does not wait on this choice: filesystem snapshots and the rest of m285 ship now, and option 1 is filed as a separate follow-up.

### D6 — Isolation runtime and Docker in a sandbox

Render documents "its own Firecracker microVM" with **Docker and Docker Compose supported**. The isolation technology itself (Firecracker vs gVisor) is not a customer-visible parity row. Docker in a sandbox is, and the research did not establish how far Render's "supported" goes (whether `docker build` works, whether published ports work, how Compose networking behaves).

**Correction (2026-11-08):** an earlier draft of this ADR said gVisor cannot run a Docker daemon. That was unverified and wrong. gVisor documents running `dockerd` inside a `runsc` sandbox ([Docker in gVisor](https://gvisor.dev/docs/tutorials/docker-in-gvisor/)). What it takes, and what it costs:

- **Node-level `runsc` flags.** `--net-raw` for every supported Docker version, plus `--allow-packet-socket-write` for Docker v28 and later. gVisor's own warning: the second flag "allows sandboxed code to craft arbitrary network packets". These are runtime-wide settings in the node's containerd `runsc` configuration, not per sandbox, so they apply to every pod on the node pool unless sandboxes get a second runtime class.
- **In-sandbox capabilities.** The tutorial uses `sys_admin`, `net_admin`, `net_raw` and `setuid` (it starts with `--cap-add all`). They are perceived by the sandboxed application only; gVisor never runs with host capabilities. They may conflict with bex's baseline Pod Security Standard on `<ws>-sandbox` namespaces; this has not been checked against the sandbox Pod template.
- **Storage.** gVisor needs `tmpfs` as the upper layer of `overlayfs`, so Docker v29 needs a `tmpfs` at `/var/lib/docker` (or `--feature containerd-snapshotter=false`).
- **Networking.** `dockerd` must run with `--iptables=false --ip6tables=false`. With iptables off, `docker run -p` and `--expose` do not publish ports (use `--network=host`), and gVisor's FAQ notes that DNS on user-defined bridge networks does not resolve. **Compose service discovery and port publishing may therefore differ from Render. Unverified.**

So Docker in a sandbox is feasible on bex's substrate, not impossible, with real trade-offs: a looser node-level network posture that has to be reconciled with the Cilium allow-list and tenant isolation (ADR042, ADR043), Pod-security exceptions, and a networking subset. It is **unverified on bex's nodes** (runsc version, kernel, node image).

**Decision:** Docker in a sandbox is a parity row, not a non-goal. It stays ⚠️ until a spike answers the questions below, and then the user signs off on whatever remains. The spike (a task for the sandbox board): start a pod on the gVisor class with the flags above and run `docker run hello-world`, a `docker build` of a small image, and a two-service Compose file with an internal call between services and a published port; record what fails; check that the Cilium allow-list and the platform floor (D3) still hold with `--net-raw` on, including with `--allow-packet-socket-write`: attempt source-IP and ARP/NDP spoofing, reach a denied destination by raw packets, and bypass the FQDN/SNI allow-list through the nested Docker's own DNS resolver. Any success rules out options (2) and (1)'s shared pool and leaves a separate pool or a signed divergence. Options after the spike: (1) a second RuntimeClass and node pool for sandboxes that opt in to Docker, keeping the default pool's flags tight; (2) enable it pool-wide if the floor holds; (3) a Kata/Firecracker pool for sandboxes, which reopens ADR042 D3 and needs `/dev/kvm`; (4) record a signed divergence in ADR018.

### D7 — Groups, plans, limits and regions

- **Plans:** `1c-2g` (default), `2c-4g`, `4c-8g`, `8c-16g`, each with a **40 GB** writable filesystem. The plans ship (w2/m263); the 40 GB figure is not yet pinned by a test or an ephemeral-storage limit. 🚧
- **Groups:** a workspace may have 10 sandbox groups; each carries a `concurrencyLimit` (default 1,000 concurrent sandboxes, lowerable per group, capped at 1,000 even if a higher value is sent); suspended sandboxes do not count; create and resume answer **429** at the limit; snapshots belong to the source sandbox's group. Region is `oregon` only, and the group sets it. **Shipped by w22/m311** ([ADR042 § Sandbox groups](ADR042-sandbox-cluster-substrate.md#sandbox-groups-w22m311)): the default group is created on first use, so every workspace has exactly one without a hook or a backfill; the limit is counted under a per-workspace lock with a slot lease, so parallel creates and resumes cannot exceed it; every resume path (explicit, exec, and the run and file token mints) is held to it; the status rule (creating, running, suspending and resuming count) is bex's reading of Render's silence; and Render's own `sandbox_group_limit_reached` (a 400, from the pinned `errorCode` vocabulary) answers the 11th group. The three numbers are configurable (`BEX_SANDBOX_MAX_GROUPS_PER_WORKSPACE`, `BEX_SANDBOX_MAX_GROUP_CONCURRENCY`, `BEX_SANDBOX_MAX_ACTIVE_PER_WORKSPACE`). The groups were the largest unscheduled parity gap and a prerequisite of snapshots (D4); m285 t002 now has its default group. Live verification is w22/m311 t007.
- **Numbers are targets, not capacity.** 10 groups, 1,000 concurrent sandboxes per group and 40 GB are Render's beta figures. bex implements them as configurable constants with its own quotas (D11); a 1,000-sandbox group on a Hetzner pool is a capacity and cost decision this ADR does not make.
- **Region:** the Python SDK accepts `region` as a compatibility parameter that does not affect placement. bex ignores it the same way.

### D8 — Pricing and billing

**Decision (2026-11-10, proposed): adopt Render's billing model. Do not record it as a divergence.** The user asked for a recommendation on whether to keep bex's own model; the parity rule says a divergence needs a reason and sign-off, and the one real reason (gVisor CPU accounting) is a spike away from being settled either way.

**What Render publishes.** The launch post ([render.com/blog/introducing-render-sandboxes](https://render.com/blog/introducing-render-sandboxes)) gives the model, and Render's own comparison article ([render.com/articles/render-sandboxes-vs-vercel-sandboxes](https://render.com/articles/render-sandboxes-vs-vercel-sandboxes), published 2026-11-06) gives the rates. `render.com/pricing` and `/docs/sandboxes*` still carry no sandbox figures.

| Item | Render |
| --- | --- |
| CPU | $0.12 per **active** CPU-hour (used, not provisioned; waiting on a model or a person adds none) |
| Memory | about $0.019 per GB-hour ($0.038/h per 2 GB), **allocated**, while running |
| Suspended | no CPU or memory charge; snapshot storage only |
| Snapshot storage | $0.08 per GB-month |

The article says APIs, defaults and limits might change before general availability. It says nothing about prices. Re-read it when ADR104 is accepted.

**What bex does today** (ADR023, ADR030): one `sandbox_compute_seconds` meter, CPU millicores plus memory folded into a weighted unit, from the **allocated shape**, running only, referenced to AgentCore ($0.0895 per CPU-hour, $0.00945 per GB-hour). There is no storage meter. Three differences follow:

| Render rule | bex today | Decision |
| --- | --- | --- |
| CPU billed on use | Allocated shape, so an idle sandbox costs what a busy one does | Adopt, after the spike below |
| Suspended pays snapshot storage only | Suspended is free; bex absorbs the rootfs image for up to 30 days | Adopt now |
| Snapshots are billed storage | No meter; m285 would add 30-day objects of up to 40 GB at no cost to the creator | Adopt now |

**Design.**

- Three meters replace the single weighted one for new usage:
  - `sandbox_cpu_seconds`: used CPU from Prometheus (`increase(container_cpu_usage_seconds_total)`), per running interval.
  - `sandbox_memory_gb_seconds`: allocated GiB × running seconds. The data is already in the phase cursor.
  - `sandbox_snapshot_gb_seconds`: bytes × retention seconds for a suspended sandbox's rootfs image and for each snapshot. It is event-driven: it starts when the commit succeeds and ends when the image is deleted (D11's GC rule: terminate, 30-day expiry, snapshot delete or expiry).
- `sandbox_compute_seconds` stays for history and for the agent-session cost in `outcome.cost` (w9/m178), so that cost does not change basis. Nothing is re-priced retroactively.
- **Rates** follow the existing `pricing.yaml` convention of Render × 0.70: $0.084 per active CPU-hour, $0.0133 per GB-hour, $0.056 per GB-month of snapshot storage. They live in `pricing.yaml`, not in code. The convention is ADR030's to change.
- **Identity.** cAdvisor series carry the pod name, and a resume recreates the pod. Record the pod name on each running interval and attribute by interval. Do not add pod labels to kube-state-metrics: its allowlist is nodes-only today, and tenant pod labels would add cardinality for no gain.
- **Coverage.** The `kubernetes-cadvisor` job lists every node, so the tainted sandbox pool is scraped. If Prometheus is unreachable, defer the window and mark it `partial`; never bill zero.
- **Surfaces.** `GET /v1/usage`, GraphQL and MCP return the new kinds together, and the Stripe line items follow ADR040.

**Spike (gates only the CPU meter).** Run `stress-ng` at one core for 60 s in a gVisor sandbox and read the cAdvisor series. Pass: within ±5% of one CPU-minute. Also measure an idle sandbox's baseline and a syscall-heavy load against the same load under runc. gVisor's sentry spends host CPU emulating syscalls, so that load reads high, and Render's Firecracker has no such cost.

- **Policy for the overhead: bill the measured cgroup CPU, with no subtraction.** It is the figure a customer can reproduce, and the 0.70 factor absorbs some of it. A variant that subtracts the sentry overhead is a separate follow-up.
- **Escalate to the user** if a syscall-heavy load reads more than 2× its runc figure, or an idle baseline exceeds 2% of one core. Either would make the bill visibly worse than Render's for the same work. These are starting thresholds; the spike task fixes them before the run.
- **If the spike fails**, keep the allocated-shape CPU charge as a time-boxed divergence recorded in ADR018, with the user's sign-off and a dated task to remove it. The storage and memory meters ship regardless.

**Not settled.**

- Whether Zot or OpenSandbox can report a snapshot's compressed byte size. Neither the sandbox package nor `deploy/opensandbox` has a size field, so this needs reading Zot's search API. Layers shared between a sandbox's successive commits will be counted more than once unless the API dedupes; charge the sum and say so.
- Whether `resuming` time is billed. Render does not say. Keep it free.
- Whether a running sandbox that still holds its old rootfs image keeps paying storage for it. The meter follows whatever D11's GC rule retains.
- Whether Hobby workspaces pay for sandbox usage. `pricing.yaml` keeps Hobby plans free, ADR091 charges Hobby per use for agent models, and Render's pages do not say. Decide it in ADR030 when the meters land; D11's caps apply either way.
- The Hobby timeout cap (D2).

### D9 — The SDK is part of the contract

Render's SDKs (`@renderinc/sdk` as `new Render().sandboxes`, and Python `Render`/`RenderAsync`) default `baseUrl`/`base_url` to `https://api.render.com`, read `RENDER_API_KEY` (and `RENDER_WORKSPACE_ID` for `owner_id`), and expose create, exec, upload, download, list, get, terminate, snapshots and groups, with **no explicit suspend, resume or stop method** (suspend and resume are implicit). Pointing `base_url` at bex-api is the drop-in hook, so the SDK walk (w32/m284) is the acceptance test for every row above, not an extra. Since the first draft, w32/m299 closed the relative-remote-path and mint-during-resume gaps; the full walk is blocked on QA credentials.

### D10 — What left early access, and the CLI pin

Checked against `render-oss/cli` release notes and `cmd/` on 2026-11-08. Only **sandboxes and sandbox-groups** have left the CLI's `ea` group (v2.32.0, breaking, no alias). `ea blueprints create|plan|approve|update` (public alpha, v2.35.0) and `ea objects` (object storage) are still early access, so bex's `[~]`/`[-]` rows for them (cli-compatibility-checklist, ADR018 row for object storage) stand. Leaving `ea` is not GA: the Sandboxes product is still labeled **Beta**. Release notes older than v2.30.0 were not read, so this is not a claim about earlier pins.

The latest CLI is **v2.36.0 (2026-11-06)**; bex pins v2.35.0. v2.36.0 adds `sandboxes create --name`, errors that explain when a sandbox ended and when, and drops the Hobby 24 h wording from `--timeout` help (the ADR018 row for `timeoutSeconds` says the Hobby cap is 86400 s; recheck it against Render's docs). `main` also shows `sandbox-groups` `delete` and `default set`, `sandboxes snapshots` (create, delete, get, list) on the CLI reference page, and `sandboxcredentials*`. All of these land on gaps this ADR lists (groups, snapshots, credentials), so the pin bump should be filed with them and graded in the checklist.

### D11 — Resource and abuse bounds

Parity widens what a sandbox can do: `allow-all` egress by default, 30-day retention of suspended sandboxes and snapshots, and a Hobby plan. Render's own numbers are not a substitute for bex's cost and abuse controls, so the following are requirements for the milestones that ship these rows, whatever Render does:

- **Storage:** a cap on snapshots per group and on total sandbox storage (suspended rootfs images plus snapshots) per workspace, a garbage-collection rule that deletes images when a sandbox terminates, expires at 30 days, or a snapshot expires, and a gauge for the registry growth the sandbox path causes.
- **Compute and egress:** the 429 concurrency limit (D7) per group and per workspace, egress metering on `allow-all` sandboxes (the egress-meter exists), and an alert on a workspace whose sandboxes egress far above its plan. Free (Hobby) workspaces get the same default; the control is the meter and the cap, not a different default.
- **Privileged capture path:** the constraints in D4.

## Gaps, owners and order

| # | Gap | Owner |
| --- | --- | --- |
| 1 | Prove D2 live on production (D3 is proved: w32/m283 is done) | w2/m278 t009, blocked on the D5 decision and a production walk |
| 2 | Sandbox groups: default group per workspace, `concurrencyLimit`, 429, 10 per workspace | w22/m311 t001–t005, and t006's workspace cap, shipped in code; the production walk with the unmodified CLI and the ledger flip are t007 |
| 3 | Filesystem snapshots and restore, including policy inheritance and D4's capture-path constraints | w32/m285 (blocked behind m278) |
| 4 | Pin `bash -c`, list `limit` ≤ 100 and 40 GB storage with tests | **new task** in w32; m284 is blocked on QA credentials, so do not wait for it |
| 5 | SDK walk grades every method | w32/m284 (blocked on QA credentials) |
| 6 | Memory/process preservation (D5) | user sign-off, then a follow-up |
| 7 | Docker/Compose inside a sandbox (D6): spike on a gVisor node, then pick an option | **new task** (spike), then user sign-off |
| 8 | Billing (D8), in order: snapshot-storage meter and memory split (no dependency); gVisor CPU spike, then the used-CPU meter; `pricing.yaml` rates, usage surfaces and Stripe lines; amend ADR023 and ADR030 | **new tasks**; sign-off only if the spike fails |
| 9 | Hobby timeout cap: confirm Render's current rule, then sign off or lift (D2) | **new task**, w2 |
| 10 | Pin is Render CLI v2.35.0; v2.36.0 (2026-11-06) is out (D10) | bump task, w2 CLI train |
| 11 | Amend ADR018's sandbox rows that file groups, images and snapshots as omissions or non-goals (§ _Render sandbox groups and snapshots_, and the `sandboxGroupId`, `buildId`/`buildName` and `snapshotId`/`snapshotName` rows of § Sandbox create flags) and the checklist's matching `[-]` rows (`sandboxes snapshots`, `--group`, `--image`, `sandbox-groups`), which this ADR contradicts (D1). Cite them by name: line numbers have drifted | same change that accepts this ADR |
| 12 | `group` on create and list (D2). `env` (w32/m297) and `name` (w5/m287) are done | w22/m311 t003 shipped in code (REST, GraphQL and MCP); verified live in t007 |
| 13 | Images/builds (`buildId`, `/sandbox-groups/{id}/builds`) refused today | follow-up, after the REST schema is confirmed |
| 14 | Storage, compute and egress bounds (D11) | with gaps 2 and 3. **Compute: done in code** by w22/m311 t004 and t006 (the per-group and per-workspace 429, `BEX_SANDBOX_MAX_ACTIVE_PER_WORKSPACE`). **Egress: open.** The operator's egress meter selects pods by `app.bex.co/app` and `bex.co/app-id`, which a `<ws>-sandbox` pod does not carry, so sandbox egress is not metered and no alert exists (ADR042 § Sandbox groups); the fix is an operator change to the meter's pod selector, then the alert, its promtool test and its Grafana panel. **Storage: open** (w22/m313 t004, w32/m285) |
| 15 | Drift watch: re-diff Render's sandbox docs, limits and OpenAPI on every CLI pin bump and at least monthly during the beta, through ADR018's re-baseline rounds; freeze the SDK/OpenAPI shapes as contract fixtures like `lego/cli/testdata` | w2 |

## Out of scope

Render's **scoped API keys for sandbox tasks** and **credential brokering** are labeled _Early access_ on render.com/sandboxes, and **Shared filesystems** is also labeled _Early Access_. They sit outside the parity bar until Render ships them out of early access; ADR062 (sandbox credential vault) covers bex's own design. **Caveat:** the CLI source on `main` already has `sandboxcredentials*` commands (create, get, list, update, delete, versions, write, restore), which the pinned v2.35.0 does not, so credentials may be further along than the product page says. Re-read the label and the commands when the pin moves (D10) and move this row into D7-style parity if it is no longer early access.

## Open questions

1. What are the exact `/v1/sandboxes` and `/sandbox-groups` schemas for groups, images, builds and snapshots, and the error format? Diff the pinned OpenAPI copy (`lego/backend/internal/api/openapi/render-public-api-1.json`) and the render-oss/sdk source against the live spec.
2. Does Hobby keep a 24 h timeout? Render's rates are answered in D8.
3. Does the D6 spike show Docker and Compose working on bex's gVisor nodes, and which option follows (second runtime class, pool-wide, Kata/Firecracker pool, or a signed divergence)?
4. Does the user accept filesystem-only suspend and snapshots (D5), or build the checkpoint?

## Consequences

- One place answers "is sandbox Render-parity done?": the D2–D7 rows, with an owner for every non-green one.
- Groups surface as a prerequisite of snapshots, which the m285 plan had not counted.
- No behavior changes in this ADR. It proposes board work (gaps 2, 4, 7, 8, 9, 14 and 15) and user decisions on D5 and, after its spike, D6. D8 is decided in favor of Render's model; the user signs off only if its CPU spike fails. Accepting it amends the ADR018 rows it contradicts (gap 11); the user's sign-off is still what makes any divergence stand.
- The billing model is a larger parity gap than the earlier draft said: bex bills allocated CPU, Render bills used CPU, and neither side of the suspended-storage rule matches. D8 closes it by adopting Render's model.

## Alternatives considered

- **Keep only the ADR018 ledger and the milestones.** Rejected: that is how the slices drifted (m278, m283, m285 each re-reading Render's docs) and how groups and the policy-inheritance rule were missed. The contract needs one place; the ledger keeps the state.
- **Treat Render sandboxes as a CLI-only subset.** Rejected: the SDKs and REST are Render's documented path, and the parity rule applies to every customer-facing behavior.
- **Build the Firecracker/Kata pool now for full microVM parity.** Not chosen: needs `/dev/kvm` (ADR042 D3) and would absorb D5 and D6 at once; kept as option (3) in D6 after the spike.

## Sources

Fetched 2026-11-08: render.com/sandboxes, /docs/sandboxes, /docs/sandboxes-limits, /docs/sandboxes-snapshots, /docs/sandboxes-sdk-typescript.md, /docs/sandboxes-sdk-python, /docs/sandboxes-creating, /docs/sandboxes-executing, /docs/sandboxes-cli-reference, the Render CLI releases (v2.30.0–v2.36.0), `cmd/` on `main` and render-oss/sdk on GitHub, render.com/changelog, and gVisor's Docker-in-gVisor tutorial and FAQ (D6), Render's launch post for D8, and render.com/articles/render-sandboxes-vs-vercel-sandboxes (2026-11-06) for the rates. All single-vendor, model-reviewed and unconfirmed by Render; state the date when citing. Secondary pricing trackers were consulted and only used to locate the first-party article. On 2026-11-10 the D8 article was read in full (published 2026-11-06), render.com/pricing and /docs/sandboxes were re-read and carry no sandbox prices, and the repo state (boards, ADR018, the CLI pin v2.35.0) was re-checked.
