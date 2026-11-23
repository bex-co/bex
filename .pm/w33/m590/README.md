# w33 · m590 — A build that runs out of memory fails fast as a tenant OOM, with Render's `build_ended` reason and accurate SLO and billing

**Worker:** worker33 **Goal:** a build killed for exceeding its 8 GB memory limit fails on its first attempt (no 2× backoff retry), tells the user the build ran out of memory instead of blaming an infrastructure reason, emits Render's `build_ended.reason.failure.oomKilled` across REST, GraphQL and MCP, stops counting as an infrastructure failure in SLO metrics, and bills `build_seconds` as a failed tenant build. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Fail build Jobs fast on OOM (exit 137 after `DisruptionTarget: Ignore`), classify as `FaultTenant`, and surface structured OOM status on `App.Status` | 1h | — |
| [t002](t002.md) | Report a memory-limit failure message (both container `OOMKilled` and child exit 137 in BuildKit) and bill `build_seconds` as a tenant failure | 45m | t001 |
| [t003](t003.md) | Populate `build_ended.reason.failure.oomKilled` (`memoryLimit`) across REST, GraphQL and MCP | 45m | t001 |
| [t004](t004.md) | Render parity | 30m | t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 45m | t004 |
| [t007](t007.md) | Closeout (live check on production) | 30m | t005, t006 |

Total: **7 tasks, about 4h30m**.

## Definition of done

1. A build whose `buildkit` (or `clone`) phase is `OOMKilled` (exit 137) fails on attempt 1 without spending `backoffLimit` retries, while genuine `DisruptionTarget` node evictions are still ignored and retried.
2. The deploy failure message and final build log state that the build ran out of memory (naming the 8 GB limit and actionable guidance) instead of `"build failed for an infrastructure reason … trigger a new deploy to retry"`. This holds for both container-level `OOMKilled` and a BuildKit `RUN` child killed with exit 137.
3. The operator classifies build OOM as `FaultTenant` (`user_failed`, `ReasonBuildFailedUserError`, `app.bex.co/build-fault=tenant`), so it does not increment `infra_failed` / correlated infra-failure alerts, and `build_seconds` bills the build as a failed tenant build (matching ADR030 §13).
4. `build_ended` carries `reason.failure.oomKilled.memoryLimit` (matching Render's OpenAPI `Build Ended` schema) on REST, GraphQL and MCP.
5. Replayed on production with a disposable `bex-canary` fixture that exceeds 8 GB during its build command (then deleted): 1 attempt, OOM failure message, `build_ended.reason.failure.oomKilled` populated.

## Source + Goal linkage

- **Source:** Diagnosing a failed Vite SSR build in the first-party `bex` workspace (`2026-11-23`), filed via `/pmx them to w33`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and build-plane reliability ([ADR060 D2](../../../docs/ADR060-build-worker-reliability-and-performance.md#d2--failure-classification-distinct-exit-codes--podfailurepolicy-retry-only-what-retrying-can-fix)).
- **Expected outcome:** Deterministic build OOMs fail in one attempt with an accurate memory-limit diagnosis, clean platform SLOs, and accurate pipeline-minute accounting.
- **Why now:** Today every build OOM contradicts ADR060 D2 by burning 2 extra 8-minute retries, telling users to retry an "infrastructure" fault, paging/polluting infra-failure SLOs, and zeroing `build_seconds` billing.
- **Render parity task included:** Changes tenant-facing build failure wording and the `build_ended` event's `reason.failure.oomKilled` payload across REST, GraphQL and MCP.
