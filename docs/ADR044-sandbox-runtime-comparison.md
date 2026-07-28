# ADR: Sandbox runtime comparison — AgentENV vs. OpenSandbox

**Status:** survey — a side-by-side read of the two published architecture documents to support sandbox-runtime technology selection. It does not reopen [ADR014-sandboxes.md](ADR014-sandboxes.md) (bex already builds on OpenSandbox); it records what each system actually is, where they converge, where they differ, and what that means for bex.

Sources (read 2026-07-27):

- [AgentENV — System Architecture](https://kvcache-ai.github.io/AgentENV/internals/architecture.html)
- [OpenSandbox — Architecture](https://open-sandbox.ai/architecture/)

## TL;DR

Both are E2B-influenced sandbox platforms with the same skeleton — lifecycle control plane, in-sandbox exec daemon, pause/resume, warm pools, per-sandbox egress policy, sandbox-ID-based reverse proxy. They differ at the **substrate**: AgentENV is a **Firecracker microVM** system whose core innovation is a Rust storage stack (overlaybd layered block devices over ublk) giving true **memory-snapshot resume**; OpenSandbox is a **container** system (Docker / Kubernetes pods) whose strength is a **complete product surface** (SDKs, CLI, MCP, code interpreter, egress/ingress components) and **Kubernetes-native** multi-node via CRDs. For bex — a Go, Kubernetes-operator shop already integrated with OpenSandbox — OpenSandbox remains the right runtime; AgentENV is worth tracking as the reference design for microVM isolation and memory snapshots, both of which OpenSandbox can approach through `RuntimeClass` secure runtimes but not fully match today.

## What each system is

### AgentENV (kvcache-ai)

A single-node-first Firecracker sandbox server. One Rust binary per Linux host with `/dev/kvm` runs an Axum HTTP API, an orchestrator (sandbox lifecycle state machine), and Firecracker VM management. Its stated core is the **storage subsystem**:

- **overlaybd** — an LSMT-based layered image format (immutable zstd-compressed read-only layers + one writable upper; segment-index reads; pluggable backends including an OCI registry backend for remote layer fetch).
- **ublk** — async userspace block devices (`/dev/ublkbN`) over Linux ublk + io_uring, managed by a dedicated daemon process (`uvm-ublk-daemon`) behind a Unix socket.
- **Memory snapshot restore** — Firecracker diff snapshots are packaged into overlaybd layers and served as a read-only ublk block device that Firecracker `mmap`s as its memory backend; pages CoW into anonymous memory on first write. Sandboxes restored from the same snapshot **share one memory device (refcounted), so the host page cache is shared** — a real density win for many-sandboxes-from-one-template.
- Node extras: network-namespace slots with warm pools, per-sandbox egress allow/deny (CIDR/IP/domain), warm Firecracker process pools, an E2B-compatible node API (`POST /sandboxes`, pause/resume, `/proxy`), node observability, and an optional iroh-based P2P artifact transport between nodes.
- A **prototype** distributed control plane in Go: gateway (HTTP reverse proxy routing by sandbox ID / `{port}-{sandboxID}.{domain}`) + scheduler (gRPC; round-robin/random; static or Kubernetes EndpointSlice discovery). All sandbox→node bindings are **in-memory** — lost on scheduler restart, rebuilt from heartbeats. Deploys to Kubernetes as a **privileged DaemonSet** (one runtime pod per host, `/dev/kvm`, iptables).

### OpenSandbox (alibaba)

A general-purpose sandbox **platform** organized in six surfaces:

- **Client surface** — SDKs (Python, JS/TS, Kotlin, C#, Go), the `osb` CLI, an MCP server, and code-interpreter SDKs.
- **Protocol surface** — OpenAPI contracts under `specs/` as the source of truth (lifecycle, diagnostics, execd, egress).
- **Lifecycle control plane** — a Python FastAPI server: API-key auth, validation, SQLite-backed persistence (snapshot records), endpoint resolution, and a server proxy (HTTP/WebSocket) with optional renew-on-access.
- **Runtime backends** — Docker (single-host; container pause/resume; snapshots as committed local images) and Kubernetes (workload providers: `BatchSandbox` CRD controller by default, or `kubernetes-sigs/agent-sandbox`). K8s pause/resume for single-replica BatchSandboxes is **rootfs commit to an OCI image + recreate**, preserving the sandbox ID. The controller also ships `Pool` (pre-warmed capacity) and internal `SandboxSnapshot` records.
- **Sandbox data plane** — the user container plus injected `execd` (Go/Gin: commands with SSE streaming, background logs, bash sessions, PTY over WebSocket, file ops, Jupyter code contexts, metrics), volumes (`host` / `pvc` / `ossfs`), and an optional egress sidecar (FQDN/wildcard rules, DNS + nftables modes, runtime `PATCH /policy`).
- **Network/security plane** — an ingress gateway (header/URI/wildcard-host routing), secure-access endpoint credentials, resource limits, capability drops, and RuntimeClass hooks for secure container runtimes.

## Where they converge

The two architectures are remarkably aligned in shape — evidence these are the load-bearing ideas of the category, not one vendor's quirks:

- **Control plane / data plane split.** Both keep lifecycle orchestration in a server and push in-sandbox work to an injected daemon (OpenSandbox `execd`; AgentENV talks to `envd` inside the VM).
- **Snapshot pause/resume + warm pools as the two latency levers.** Both treat "restore, don't boot" and pre-warmed capacity as separate mechanisms — the same split [ADR014-sandboxes.md](ADR014-sandboxes.md) D5/D6 records.
- **Per-sandbox egress policy**, mutable at runtime (AgentENV `allowOut`/`denyOut` endpoint; OpenSandbox egress sidecar `PATCH /policy`).
- **Sandbox-ID-routed reverse proxy** for reaching services inside sandboxes, including `{port}-{sandboxID}.{domain}` host-based URLs in both.
- **E2B as the reference point.** AgentENV's node API is explicitly E2B-compatible (`POST /sandboxes`, `/pause`, `/resume`); OpenSandbox's lifecycle spec covers the same verbs, and bex maps E2B shapes onto it at the gateway (ADR014 D2).
- **Kubernetes as the multi-node answer — reluctantly.** AgentENV's scheduler can discover nodes via EndpointSlice and deploys as a DaemonSet; OpenSandbox delegates multi-node to k8s entirely. Neither invents a non-k8s cluster fabric for production.

## Where they differ

| dimension | **AgentENV** | **OpenSandbox** |
| --- | --- | --- |
| isolation substrate | **Firecracker microVM** (requires `/dev/kvm`; privileged DaemonSet on k8s) | **containers** (Docker / k8s pods); secure runtimes (Kata etc.) optional via RuntimeClass |
| pause/resume semantics | **memory + fs snapshot** — Firecracker diff snapshot → overlaybd layers → ublk-mmap restore; same-instance resume, page-cache shared across same-template sandboxes | Docker: container pause (RAM held, host-pinned); k8s: **rootfs-only** commit to OCI image + recreate (no memory state) |
| image/storage stack | custom overlaybd LSMT layers over ublk; lazy OCI-registry backend; zstd; reflink CoW snapshots | plain OCI images + containerd/Docker; volumes (`host`/`pvc`/`ossfs`) |
| implementation language | **Rust** node + storage; Go prototype control plane | **Python** server; Go controller/execd/ingress/egress |
| multi-node control plane | prototype gateway + scheduler, **in-memory bindings**, round-robin/random | **Kubernetes-native**: BatchSandbox/Pool/SandboxSnapshot CRDs + controller; k8s scheduler does placement |
| product surface | node API (E2B-compatible) + admin/node APIs; no SDKs/CLI/MCP in the doc | SDKs ×5 languages, `osb` CLI, **MCP server**, code-interpreter (Jupyter) images, diagnostics API, secure-access credentials |
| state durability story | snapshot repository (POSIX/OSS backends) + P2P artifact fetch; paused-sandbox persistence across node restarts | server SQLite for snapshot metadata; k8s API for workload state |
| maturity signal | control plane self-described as **prototype**; single-node server is the real product | shipped multi-component platform; already integrated in bex (`deploy/opensandbox`, `BEX_RUNTIME=opensandbox`) |

Three differences dominate everything else:

1. **Memory snapshots vs. rootfs snapshots.** AgentENV resumes a sandbox with its processes and RAM intact — true E2B-class hibernation, with cross-sandbox page-cache sharing as a density multiplier. OpenSandbox's k8s path preserves only the filesystem (the Docker path preserves RAM but pins the sandbox to its host and keeps the memory occupied). Anything bex documents as "memory survives pause" is true only on the Docker runtime today ([ADR014-sandboxes.md](ADR014-sandboxes.md) D5/D7 state ladder).
2. **MicroVM vs. container isolation.** AgentENV's threat boundary is a hypervisor by default. OpenSandbox's is a container, with secure runtimes (Kata/Firecracker-class via RuntimeClass) as an opt-in — which closes much, but not all, of the gap (a Kata pod still has container-style snapshot semantics underneath OpenSandbox's pause/resume).
3. **Platform completeness vs. substrate depth.** OpenSandbox ships the surrounding product bex would otherwise have to build (SDKs, MCP server, execd, egress sidecar, ingress, pools). AgentENV ships a deeper storage/isolation substrate but only a prototype control plane and no client surface — adopting it would mean bex builds (or keeps building) the gateway, MCP tools, and client ergonomics itself, and operates a Rust codebase plus a parallel scheduler next to Kubernetes.

### The memory-density mechanism, precisely: shareable mmap, not lighter VMs

A tempting misread of "Firecracker + memory snapshots" is that each AgentENV sandbox is individually lighter than a container. It is not. A microVM carries a whole guest kernel and a minimum guest-RAM allocation (e.g., a ~128 MiB floor); a container is just a namespaced process with near-zero substrate tax. **Per unit, in isolation, a container is lighter — the win is density, not volume.**

The density comes from Linux's ordinary copy-on-write, used in the right place:

1. **The snapshot becomes a shareable read-only memory backend.** Firecracker's memory diff snapshot is packaged as an overlaybd read-only layer, exposed as a ublk block device (`/dev/ublkbN`), and `mmap`'d by Firecracker as its guest physical-memory backend.
2. **Many VMs mmap the same device → the host kernel shares the clean pages.** Because every same-template Firecracker process maps the same read-only block device, the host page cache holds one copy of each read-only page shared across all of them (`MAP_PRIVATE` semantics — the CoW itself is stock Linux, not the novel part).
3. **Writes are page-granular (4 KiB).** Only a page a sandbox actually writes triggers a fault and is copied into a private anonymous page; everything else stays shared. The marginal cost of the Nth same-template sandbox is therefore near-zero — the same page-sharing logic that drove Firecracker's selection for AWS Lambda.

The economics only flip when **many sandboxes come from one template**; for heterogeneous, long-lived, or few-sandbox workloads the microVM's fixed tax dominates and containers stay cheaper.

This also clarifies why OpenSandbox cannot match it _within its current substrate_ — not merely because the feature is absent. Containers have independent address spaces: there is no "multiple processes mmap the same read-only memory file" premise for the kernel to share against. Docker pause keeps each sandbox's RAM occupied and host-pinned (no sharing); the k8s BatchSandbox resume drops memory and re-boots (no warm memory to share). The missing piece is not CoW — it is a shareable, mmap'd, read-only memory backend, which requires a hypervisor that can treat a block device as guest memory.

## Implications for bex

Context: [ADR014-sandboxes.md](ADR014-sandboxes.md) already selected OpenSandbox as the sandbox runtime (pillar 5), with bex-api as an E2B-shaped synchronous gateway over it. This comparison supports that choice and sharpens two known gaps:

- **Stay on OpenSandbox.** The deciding factors are organizational, not just technical: bex is a Go/Kubernetes-operator codebase (all product Go in `lego/`, day-1+ state in `deploy/gitops/`), and OpenSandbox is the only one of the two that is Kubernetes-native (CRDs + controller bex already deploys via `deploy/opensandbox/`) and ships the client/MCP surface bex's AI-native story needs. AgentENV's Rust node + prototype in-memory control plane would trade a working integration for a substrate rewrite and a second, non-k8s scheduler to operate.
- **Memory-snapshot gap is real but bounded.** ADR014's "filesystem **+ memory** survives pause" claim holds only on the Docker runtime; on Kubernetes (BatchSandbox) pause/resume is rootfs-only. If true hibernation on k8s becomes a hard product requirement (iterative agent sessions that keep REPL/process state — the ADR014 "iterative session" edge), AgentENV's ublk/overlaybd memory-restore design is the prior art to study, and containerd checkpoint/restore (CRIU) is the k8s-native path to watch — not a reason to switch runtimes.
- **Isolation gap has a k8s-native answer.** ADR014 already flags untrusted repo code as "the strongest case for microVM isolation (Kata / Firecracker on bare-metal)". OpenSandbox supports secure runtimes via RuntimeClass, so bex can raise isolation **within** the current runtime when that milestone lands; AgentENV-grade default-microVM isolation is the fallback argument only if RuntimeClass integration proves insufficient.
- **Track, don't adopt, AgentENV.** Revisit if (a) its control plane graduates from prototype (durable bindings, real scheduling), (b) bex's sandbox roadmap makes memory-snapshot density (page-cache sharing across same-template resumes) a measured bottleneck, or (c) a microVM-default product requirement emerges that RuntimeClass cannot satisfy.

## Consequences

- No code or deployment change: OpenSandbox remains the sandbox runtime per ADR014; this ADR only records the evaluated alternative.
- The ADR014 docs claims about memory-preserving pause should continue to be read as Docker-runtime-only (as D5/D7 already state); this ADR adds the external evidence for why.
- Two watch items for the sandbox roadmap: (1) k8s-native memory checkpoint/restore (containerd/CRIU) as the path to close the hibernation gap without leaving OpenSandbox; (2) RuntimeClass/Kata on the Hetzner bare-metal nodes when untrusted-code isolation becomes a shipped requirement.
