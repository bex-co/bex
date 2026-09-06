# w7 · m86 — Per-App registry build cache (ADR060 D3)

**Worker:** worker7 **Goal:** a rebuild of unchanged dependencies stops paying for a full recompile, without weakening the build's credential isolation or its per-workspace boundary. **Status:** done

## Tasks (in order)

| id   | title                                                                    | est | depends_on               |
| ---- | ------------------------------------------------------------------------ | --- | ------------------------ |
| t001 | Settle the credential-isolation shape for cache export (ADR060 amendment) | 45m | — — **DONE**             |
| t002 | BuildKit `--export-cache` / `--import-cache` behind `BEX_BUILD_CACHE`     | 50m | w7/m86/t001 — **DONE**   |
| t003 | Zot per-App ACL + credential coverage for the `-cache` repository         | 40m | w7/m86/t002 — **DONE**   |
| t004 | Retention: exempt `-cache` repos and give them their own policy           | 40m | w7/m86/t003 — **DONE**   |
| t005 | Prove cache loss changes only speed, never results                        | 35m | w7/m86/t004 — **DONE**   |
| t006 | Measure hit rate and duration delta on a real repeat build                | 35m | w7/m86/t005 — **DONE** |
| t007 | Simplify the code this milestone changed                                  | 30m | w7/m86/t006 — **DONE**   |
| t008 | Test coverage for the shipped behavior                                    | 40m | w7/m86/t006 — **DONE**   |
| t009 | Closeout                                                                  | 15m | w7/m86/t007, w7/m86/t008 — **DONE** |

## Status note (2026-08-22)

The feature is implemented, tested and documented; `BEX_BUILD_CACHE=registry` is off by default and the gate-off build Job was verified byte-identical to the pre-milestone code. Three of the ADR's own premises were falsified by measurement and corrected in [docs/ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md) D3: the restored cache layout must be re-tagged or the cache is silently inert, the retention exemption is unnecessary while a tightening policy is not expressible in Zot, and digest equality across a cache wipe was never achievable because clean builds already differ.

The `/simplify` pass (t007) found three defects rather than cleanups, all fixed and pinned by tests: the pod's ephemeral-storage ceiling had silently doubled from 16G to 32G, neither skopeo cache transfer had a timeout (a hang would have failed a build whose image was already pushed), and the cache repository was never deleted when its App was — orphaning the larger artifact in a repository whose ACL had just been revoked.

**Closed 2026-09-05.** On-cluster D5 pairs completed: dashboard Dockerfile **179s → 38s (−78.8%)**, dependency-focused native Node API **75s → 64s (−14.7%)**, with zero queue time. Warm dependencies were cached and image digests matched. Incremental unique cache storage was 173,078,026 bytes and 5,825 bytes; restoring the native cache still processed about 517 MB. The production gate remains unset. The measured hit rate does not trigger persistent cache volumes; estate-wide enablement remains a capacity decision. See [the drill record](../../../../docs/drills/2026-09-05-build-cache.md).

## Definition of done

A second build of the same repository with unchanged dependencies reuses cached layers and completes measurably faster. A workspace can never read another workspace's cache. A warm cache hit reproduces the cached image; a deleted or corrupt cache still produces an equivalent usable image. Independent clean builds are not required to have identical digests (t005 measured that baseline nondeterminism). Cache changes speed, never application behavior. Image retention can never evict a hot cache, and cache growth can never evict a deployable generation. With the env gate off, behavior is byte-identical to today.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../../builder-issues.md) §3.10 (P9); [docs/ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md) D3 (rollout step 5).
- **Goal linkage:** [`.pm/GOAL.md`](../../../GOAL.md) #3 (git push to deploy) — build time is the dominant latency in push-to-deploy, and the Dockerfile and native paths have no layer cache at all today (only kpack does).
- **Expected outcome:** rebuilds of unchanged-dependency commits stop recompiling from scratch; the effect is visible in `bex_build_run_seconds`. Registry storage grows by per-App cache repos under their own retention, and each build gains a second push artifact — both visible in the D5 series.
- **Why now — the weakest why-now of this batch, recorded honestly.** `.pm/FUTURE-MAYBE.md` holds a related-but-distinct entry (build-artifact reuse for `deploy_only`) whose trigger, "build times become a user complaint", is **unconfirmed**. This is a different mechanism (a layer cache, not artifact reuse) on an actively-executing ADR rollout, which is its own authorization. **Sequence it after m83:** the 2026-08-17 measurement showed capacity is adequate, so speed is the remaining lever — and m83's queue-vs-run split is what shows whether image pulls or recompiles dominate. Deciding before that data exists would repeat the mistake `builder-issues.md` §4.1 records.
- **Render parity task omitted:** yes — this is build-plane mechanism behind an env gate. No REST, GraphQL, MCP or dashboard surface is added or changed; cache state is deliberately not surfaced (if a later change surfaces it, parity applies then).

## Closeout verification — 2026-09-05

The DoD is supported clause by clause: t006 records real metric-based acceleration; t003/t008 cover per-App grants, cross-workspace denial and shared-mode credential isolation; t005 records warm digest equality and clean fallback after cache loss/corruption; t004 records per-repository retention and the single rewritten cache tag; t002/t008 assert unchanged gate-off Jobs. Full operator tests passed after the final three-agent simplify review. Both opt-in drill packages compile and skip without configuration. All disposable benchmark Jobs, Pods and tags were removed. Physical unreferenced blobs follow normal Zot GC. No production manager setting or serving App was changed.
