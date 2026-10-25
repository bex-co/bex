# w1 · m184 — Platform-initiated rebuilds are not billed as tenant build minutes

**Worker:** worker1 **Goal:** a `deployed_by_render` rebuild (the platform-rebuild fleet rebuild that ships a runtime security patch) meters zero `build_seconds` for the tenant. **Status:** done (2026-10-25)

## Tasks (in order)

| id   | title                                                          | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | Operator stamps the release revision label on BuildKit Jobs — **DONE**    | 20m | —          |
| t002 | Usage skips build Jobs of platform-rebuild releases — **DONE**            | 45m | t001       |
| t003 | Simplify — **DONE**                                                       | 15m | t002       |
| t004 | Test coverage — **DONE**                                                  | 30m | t002       |
| t005 | Closeout — **DONE**                                                       | 10m | t004       |

## Definition of done

- Every BuildKit build Job carries `app.bex.co/build-revision` (kpack artifacts already do).
- `build_seconds` excludes Jobs whose revision belongs to a deploy opened with trigger `deployed_by_render`; tenant-triggered builds meter exactly as before.
- Tests cover both: an exempt platform rebuild and an unchanged tenant build in the same window.

## Source + Goal linkage

- **Source:** [`.pm/w1/done/137.md`](../done/137.md) (promoted 2026-10-25 by `/loopx w1`).
- **Goal linkage:** billing correctness and trust: tenants pay for work they asked for, not for the platform's own security maintenance.
- **Expected outcome:** the CVE-2026-111497 rebuild (and future platform rebuilds) adds no build minutes to tenants' invoices.
- **Why now:** `platform-rebuild` shipped and already ran once; every future runtime patch would bill tenants.
- **Render parity task omitted:** no REST/GraphQL/MCP/UI shape changes; the usage numbers the surfaces show simply stop including platform rebuilds.

## Closeout (2026-10-25)

- t001: BuildKit Jobs and their pods carry `app.bex.co/build-revision` (`buildJobLabels`, `TestBuildJobCarriesItsReleaseRevision`).
- t002: `PGStore.PlatformRebuildRevisions` reads `deployed_by_render` generations; usage's `queryBuildSeconds` skips those Jobs, and a failed lookup is an unavailable source (retried, never billed).
- t003: reviewed; follows the existing optional-capability and label-helper patterns.
- t004: exempt vs tenant build in one window, failed-lookup source, real-DB store test (`TestPGPlatformRebuildRevisionsListsOnlyPlatformReleases`, runs in the ship gate).
- Builds that ran before the operator stamps the label (including the 2026-10-25 tianpan-v4-web rebuild) carry no revision label and stay metered.
