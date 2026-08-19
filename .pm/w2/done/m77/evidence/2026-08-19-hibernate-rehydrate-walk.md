# Agent-session hibernate → rehydrate — production walk

**Date:** 2026-08-19 · **Cluster:** `hetzner-prod` · **Milestone:** w2/m77 · **Session:** `ags-da33092c0fus738gr25g`

Repo `bex-co/bex-hello-go-live`, branch `bex-agent/m77-hibernate-walk-3`. Marker file `/workspace/m77-hibernate-walk.txt` = `m77-walk-2026-08-19` (uncommitted at first hibernate; still present after every restore). Idle grace was **not** lowered cluster-wide; Completer idle was advanced with `UPDATE agent_sessions SET updated_at = now() - interval '35 minutes'` (Completer measures idle from `updated_at` / last SSH disconnect). No editor SSH was open.

## t001 / t002 — store + arm

See [2026-08-19-snapshot-bucket-provision.md](2026-08-19-snapshot-bucket-provision.md). After the env refs rolled, bex-api logs:

```
bex-api: agent-session hibernation enabled (object store bex-agent-snapshots)
```

Retention 168h, pin quota 10. Rollback: delete Secret `bex-system/bex-agent-snapshot` (`optional: true` refs) ⇒ `NewS3SnapshotStore` returns nil and reclaim stays Terminate.

## t003 — hibernate

First Completer snapshot of this session (after egress fix `e3562e56` / image pin `f1211092`):

| field            | value                                                                                          |
| ---------------- | ---------------------------------------------------------------------------------------------- |
| phase / status   | `hibernated` / `hibernated` at `2026-08-19 22:39:25Z`                                          |
| snapshot_ref     | `agent-snapshots/tea-d98210cbbpdc73dcrkvg/ags-da33092c0fus738gr25g-1787179164487314752.tgz`   |
| snapshot_bytes   | 20984                                                                                          |
| snapshot_sha     | `f170deb277c3fe042d8f44d295e9b644159aa4d6d17f76ca99f377859744f4f1`                             |
| retain_until     | `2026-08-26 22:39:25Z` (7d, clean tree)                                                        |
| sandbox / pod    | `sandbox_id` empty; `tea-…-sandbox` had no pods                                                |

`HeadObject` on that key: `ContentLength=20984`, `LastModified=2026-08-19T22:39:26Z`, empty `ServerSideEncryption` (Wasabi automatic AES-256; same as t001).

Prior failed walks (t007 input, not papered over):

1. `ags-da31i2cu2u1s7387sigg` — snapshot curl **exit 6** (Cilium NXDOMAIN of `s3.eu-central-2.wasabisys.com`). Completer: `hibernation failed, falling back to terminate`. Fixed by admitting the snapshot host on session egress (`sessionegress.SnapshotStoreDomains`).
2. `ags-da32j9neaoic73c492c0` — canceled; CNP lacked the Wasabi host (pre-fix image).

## t004 — rehydrate

Steer of the hibernated session (REST `POST /v1/agent-sessions/{id}/steer`). Restore env `BEX_AGENT_RESTORE_URL` / `_SHA` / `_BYTES` present in the sandbox; marker file restored as `m77-walk-2026-08-19`.

Instrumentation (`service.go`): `agent-session rehydrate: session=<id> resumed in <duration> (SLO p50<5s/p95<15s)`.

| cycle | when (UTC) | duration | turn outcome                                      |
| ----- | ---------- | -------- | ------------------------------------------------- |
| 0     | 22:43:48   | 3.246s   | restore ok; turn **failed** (`git ls-remote` 403) |
| 1     | 23:31:56   | 23.341s  | **completed** (first pull of new agent image)     |
| 2     | 23:33:21   | 3.123s   | **completed**                                     |
| 3     | 23:34:07   | 3.133s   | **completed**                                     |

Warm-node sample (cycles 2–3, plus the 3.246s restore that did not finish the turn): **min 3.123s / median 3.133s / max 3.246s** — inside p50<~5s. Cycle 1's **23.341s** is a **p95 miss** caused by scheduling + first pull of `ghcr.io/bex-co/bex-agent-sandbox@sha256:b2dfebe2865a3688fedc5c499526c7a4a69aac4c2c7b2d800b57ab975f3bf18f` after the git-retry image pin, matching ADR059's "cold-node resume is bounded by pre-warming the base image." No clean-re-clone fallback; marker survived every successful restore.

Cycle 0's `remote: forbidden` was the documented clone-path sandbox_id race: restore skips `cloneWithRetry` and `ls-remote`'d once while mint still saw an empty `sandbox_id`. Driver now retries those remote git calls (`withGitAuthRetry`, same 10×3s budget as clone).

## Rollback (unchanged)

Delete `bex-system/bex-agent-snapshot`. All six Deployment refs are `optional: true`. Partial Secret still fails startup (`ErrPartialS3SnapshotConfig`).
