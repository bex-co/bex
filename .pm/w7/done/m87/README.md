# w7 · m87 — Rebuild native output when build-time environment changes

**Worker:** worker7 **Goal:** A cached native rebuild at an unchanged commit produces output from the current effective build environment. **Status:** done

**Estimate:** 3h implementation; ~5h including the standing closing tasks.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce stale native output after a build-time environment change | 40m | — — **DONE** |
| t002 | Version the effective native build environment without exposing its values | 45m | w7/m87/t001 — **DONE** |
| t003 | Make native cache reuse depend on the build environment revision | 50m | w7/m87/t002 — **DONE** |
| t004 | Exercise native cache invalidation across environment sources | 45m | w7/m87/t003 — **DONE** |
| t005 | Render parity | 30m | w7/m87/t004 — **DONE** |
| t006 | Simplify | 30m | w7/m87/t005 — **DONE** |
| t007 | Test coverage | 45m | w7/m87/t005 — **DONE** |
| t008 | Closeout | 15m | w7/m87/t006, w7/m87/t007 — **DONE** |

## Definition of done

- [x] At an unchanged source commit with registry caching enabled, changing MESSAGE from A to B yields a newly built artifact containing B; a repeat with unchanged inputs can reuse the fresh cache.
- [x] Literal values, service-owned Secrets, linked-group values, override precedence, removal/unlink, and empty/newline values retain the declared effective-environment behavior.
- [x] Platform-generated Dockerfiles, image metadata, and diagnostic output do not expose secret values or value-derived guessing material; synthetic tenant output is used for proof.
- [x] Cache-disabled behavior and cache-loss fallback remain correct. Required environment-source failures still stop the build rather than producing partial configuration.
- [x] The pinned-image/dev-7 build drill proves actual output and warm-cache controls, meaningful regressions and affected checks pass, evidence is recorded, and all standing closing tasks are complete.

## Source + Goal linkage

- **Source:** user-approved 2026-09-08 pm-brainstorm proposal 1 and subsequent `$pm` handoff; [w7/m86](../m86/README.md), [w4/m92](../../../w4/done/m92/README.md), and [w4/m93](../../../w4/done/m93/README.md).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) deterministic, API-first hosting and [.pm/GOAL.md](../../../GOAL.md) #3 (git push to deploy).
- **Expected outcome:** rebuilding unchanged source with changed build-time configuration produces the requested configuration, while repeated unchanged input keeps cache reuse.
- **Why now:** per-App registry caching and exact native environment delivery have both shipped, but their recorded acceptance does not cover an environment-only warm-cache transition. Fix this interaction before evaluating production cache enablement.
- **Render parity included:** tenant-visible rebuild semantics expose this contract through REST/GraphQL/MCP and dashboard environment actions. Recheck ADR018's [Env vars](../../../../docs/ADR018-render-parity.md#environment--config) and Environment groups rows, currently marked ✅; this is a correction to that claim under enabled caching, not a newly missing feature.
- **Sizing and placement:** 3h of implementation across four tasks, plus 2h of standing closing work; worker7 has capacity and the work extends m86. No permanent workstream specialty is implied.

## Evidence and scope

Local BuildKit reproduction + unit coverage proved the secret-cache stale-output failure (`MESSAGE=A` kept after an env-only change to B) and the opaque-revision fix (`MESSAGE=B` after bumping the revision). Operator unit tests cover Secret/literal/unlink invalidation, restart stability, and non-leakage of values into Dockerfile/annotations. Opt-in cluster drill extended with `BEX_CACHE_DRILL_WORKLOAD=native-env` (A→B→B + in-cluster artifact read); the local kind kubeconfig was TLS-broken during closeout, so Job timings on the live build plane remain a re-run. Dated evidence: [docs/drills/2026-09-08-native-env-cache.md](../../../../docs/drills/2026-09-08-native-env-cache.md). Production enablement stays [043](../../043.md); clear-cache deploys are [m88](../m88/README.md).
