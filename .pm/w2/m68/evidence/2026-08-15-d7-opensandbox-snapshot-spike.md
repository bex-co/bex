# D7 spike — OpenSandbox `Suspend`/snapshot: where stored, survives Terminate?, scales? → **verdict B**

**Date:** 2026-08-15 · **Cluster:** `hetzner-prod` (read-only) + repo sources · **Task:** w2/m68/t001

## TL;DR

OpenSandbox's native snapshot is **durable past Terminate** but is **OCI-registry-backed** (an image-committer Job commits each container to an image pushed to Zot) with **per-instance image cardinality**. That meets 1 of ADR059 D7's 3 A-criteria (durability) but fails the other two (object-storage-backed; scales without per-instance manifest/tag/ACL explosion). ADR059 D3 already rules out "a general OCI registry, whose per-instance manifest/tag/ACL cardinality explodes." **→ pick B: bex self-owns a tar of the mutable mount → ADR050-encrypted object storage under a per-workspace prefix, with an initContainer hydrate on resume.**

## The three D7 questions, answered with evidence

### Q1 — Where is a suspended/snapshotted sandbox's state stored?

An **OCI registry** — `zot.bex-registry.svc:5000/snapshots` — not object storage.

Evidence (prod `opensandbox-controller-manager` args):

```
--image-committer-image=sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/image-committer:v0.1.0@sha256:d72cce22…
--snapshot-registry=zot.bex-registry.svc:5000/snapshots
--snapshot-registry-insecure=true
--snapshot-push-secret=bex-snapshot-push
--resume-pull-secret=bex-snapshot-pull
--snapshot-job-namespace=opensandbox-snapshot
--commit-job-timeout=10m
--containerd-socket-path=/run/containerd/…
```

The `SandboxSnapshot` CRD (`sandboxsnapshots.sandbox.opensandbox.io`) makes this first-class:

- `spec.sandboxName` — "Controller uses this to find BatchSandbox → find Pod → **dispatch commit Job**."
- `status.containers[]` — per-container result carrying **`imageUri`** ("the snapshot image URI for this container") and **`imageDigest`** ("digest of the **pushed snapshot image**").

So a snapshot = commit the container filesystem to an OCI image (via the image-committer against the containerd socket) and push it to the Zot registry under `/snapshots`.

### Q2 — Does the snapshot survive `Terminate`?

**Not usably — no.** Two layers:

1. **The registry artifact is durable in principle.** A commit Job pushes an image referenced by `imageUri`/`imageDigest` in `SandboxSnapshot.status`, decoupled from the pod. `bex-snapshot-pull` is mirrored into every tenant `tea-*-sandbox` namespace (confirmed: dockerconfigjson present in ~all `tea-…-sandbox` namespaces; `bex-snapshot-push` in `opensandbox-snapshot`).
2. **But bex has no restore-into-fresh-pod path, and never creates a snapshot.** bex's `Suspend` only sets `BatchSandbox.spec.pause=true` — it **pauses the SAME pod** — and `Resume` unpauses that same pod (`lego/backend/internal/sandbox/client.go:191-210`, `service.go:439-441`: "OpenSandbox resumes **the SAME pod**"). bex **never creates a `SandboxSnapshot`** (no `SandboxSnapshot`/`imageUri`/`/commit` reference anywhere in the sandbox path). Even OpenSandbox's own unpause Job is pinned to `status.sourcePodName` + `status.sourceNodeName` (`sandboxsnapshots.yaml:150-157`), so resume requires the pod to still exist. `Terminate` (`DELETE /sandboxes/{id}`) deletes the pod, and the completion path (`CancelAgentSessionSandbox` → Terminate → clear `sandbox_id`) leaves any pushed image orphaned with no wired way back.

So in bex's shipped path, `Suspend` is "pause the live pod" (still billed compute, not reclaimed), **not** "retain a restorable snapshot." A durable-past-Terminate hibernation store does not exist today.

### Q3 — How does it scale to many sandboxes?

**Poorly, by ADR059's bar.** One committed image (manifest + tag) **per snapshot per container**, pushed to the shared Zot registry; the commit Job runs in `opensandbox-snapshot` with a 10-minute timeout. This is precisely the per-instance OCI manifest/tag/ACL cardinality growth ADR059 D3 rejects ("**never** a general OCI registry … whose per-instance manifest/tag/ACL cardinality explodes"). It is also the concern raised during the ADR design ("sandbox 这么多，这么多镜像会爆炸").

## Decision rule (from ADR059 D7)

> A only if the snapshot is **durable past Terminate, object-storage-backed, and scales to many sandboxes**; anything less ⇒ B.

| Criterion                       | Native `Suspend` snapshot                                                        | Meets A? |
| ------------------------------- | -------------------------------------------------------------------------------- | -------- |
| Durable past Terminate          | No usable path — bex only pauses the same pod, never snapshots; resume is same-pod | ❌       |
| Object-storage-backed           | No — OCI registry (`zot…/snapshots`)                                              | ❌       |
| Scales without cardinality blow | No — one repo per sandbox-container + tag per snapshot + privileged Job per pause  | ❌       |

**→ Verdict: B** (all three A-preconditions fail; the earlier "durable" reading was schema-only — the code path has no restore-into-fresh-pod).

## What B looks like (for t003+)

- On hibernate: scrub credentials (`bex-pre-snapshot`, already the `Suspend` preamble), then `tar` the mutable workspace mount (working tree incl. uncommitted edits + `~/.zed_server`), `age`-encrypt per ADR050, upload to object storage under a **per-workspace prefix** (never a registry), record the object ref + size on the workspace row.
- On rehydrate: an initContainer pulls + decrypts + untars into the fresh pod's mount before the driver starts; measure resume latency against p50<~5s / p95<~15s.
- Reuse from the native path: the `bex-pre-snapshot` scrub, uid 10001 preservation, and the per-workspace prefix/ACL + ADR050 encryption invariants (non-negotiable in review).

## Notes / caveats

- Empirical verification was **schema- + config-level and read-only**. A live suspend→terminate→resume timing run against a real tenant sandbox was intentionally **not** performed on prod (it would disturb a live tenant agent session and belongs to the m68 live-acceptance step, t010, on a disposable workspace).
- The native commit path is not wasted knowledge: if a future object-storage export lands in OpenSandbox upstream, A could be revisited — but as shipped it is registry-only, so B stands.
