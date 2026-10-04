# w4 · m154 — Preserve URL path semantics in static edge rules

**Worker:** worker4 **Goal:** static redirects preserve captured filenames, and local rewrites serve the object identified by an accepted destination URL. **Status:** done — 2026-10-02; all 7 tasks done. Deployed live URL acceptance passed ([live acceptance](live-acceptance.md)).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Escape captured paths in redirect destinations | 45m | — |
| t002 — **DONE** | Resolve rewrite URLs to the intended local object | 35m | w4/m154/t001 |
| t003 — **DONE** | Cover shared callers, aliases and existing controls | 30m | w4/m154/t001, w4/m154/t002 |
| t004 — **DONE** | Verify Render parity and document destination semantics | 15m | w4/m154/t003 |
| t005 — **DONE** | Simplify the URL boundary changes | 15m | w4/m154/t004 |
| t006 — **DONE** | Test real HTTP path and redirect behavior | 35m | w4/m154/t004, w4/m154/t005 |
| t007 — **DONE** | Closeout after live URL acceptance | 10m | w4/m154/t006 |

Total: **7 tasks, about 3h 5m**. This ships a QA filing, not a product fix.

## Definition of done

Repeat the Free static-site fixture and saved rows in [finding.md](finding.md), waiting for the resolver refresh after each save.

- Under `/qa-jump/* → /docs/:splat`, GET/HEAD of `/qa-jump/a%3Fb.yaml`, `a%23b.yaml`, `a%25b.yaml` and `a%20b.yaml` produce valid same-origin Locations whose path still contains the literal filename characters (encoded as `%3F`, `%23`, `%25`, `%20`). The browser probe of the hash case keeps `a%23b.yaml` in the pathname instead of navigating to `/docs/a#b.yaml`.
- Change that destination to `/docs/*`, reload, and repeat the tested question/hash cases: the same preserved-path outcome holds. Plain `/qa-jump/plain.yaml` remains `301 /docs/plain.yaml`.
- Save `/qa-rewrite.yaml → /render.yaml?from=rule` and `/qa-encoded.yaml → /%72ender.yaml` as rewrites. Both GETs return the identical published YAML bytes/hash as direct `/render.yaml?from=rule` and `/%72ender.yaml`; HEAD is 200 and neither rewrite redirects. Changing the first destination to plain `/render.yaml` still works.
- Keep the observed controls: `/qa-copy/render.yaml?incoming=one` serves the YAML via `/:splat`; redirect-before-rewrite on `/qa-order` is 301 to `/render.yaml`; the existing `/render.yaml` file defeats its configured redirect; a literal redirect destination `/render.yaml?from=rule#frag` retains those configured URL components.
- Fresh UI, REST route GET, GraphQL service.routes and MCP list_static_routes retain the accepted ordered rows. Delete the owned site through its typed confirmation; list/API/public endpoint become absent, exact resource/Secret inventory empties, and the verification session is revoked.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, sweep 16 on 2026-10-02, using the user-selected `muse.env` and w4. [finding.md](finding.md) contains complete requests/responses, two page lifetimes, hard controls and cleanup evidence.
- **Goal linkage:** ADR008 reliable managed hosting; [ADR029](../../../docs/ADR029-static-sites.md) object-backed static serving and edge rules; [ADR018](../../../docs/ADR018-render-parity.md) Render-compatible static rules.
- **Expected outcome:** accepted routes preserve URL path data and resolve the same local content a direct destination request serves.
- **Why now:** ordinary saved rules succeed across all APIs while public visitors receive a wrong path or 404. No rebuild is needed to enter the defect.
- **Render parity included:** tenant-facing public HTTP behavior changes; compare the documented destination/wildcard contract while retaining bex's local-only and implicit-SPA decisions.
- **Dedupe:** uncovered URL-boundary gap from w1/m21, not a recurrence of w4/m94 file precedence, w4/m101 error headers, or w4/m150 header-pattern grammar. No matching open item or pending-main static-server fix was found.
- **Limits:** Free/public platform host, one no-build static fixture, dashboard writes plus three-surface reads. Actual special-character destination files, custom domains, Blueprint/REST/MCP writes and malformed/hostile URL combinations were not exercised live; t003/t006 own them. All own resources and both interrupted/resumed sessions were cleaned up.

## Drain verdict — 2026-10-02

Implementation complete with [local verification](verification.md). **Blocked:** release pipeline must deploy static-server, bex-api and dashboard; QA must replay the owned Free fixture GET/HEAD/browser and API/UI checks, then delete the fixture and revoke its session. t006 retains deployed acceptance; t007 retains closeout. No hosted fixture or session was created by this implementation run.

## Independent supplemental verification

The parallel drain retained URL implementation `703068251` and added pre-allocation expansion bounds, real-wire and existing-App Blueprint regressions. [Supplemental evidence](supplemental-verification.md) includes the local Chrome navigation proof and merged checks.

## Live acceptance — 2026-10-02

On deployed static-server `1263d12ae` (includes `703068251`/`966328891`), a fresh Free site redirected `/qa-jump/a%3F|%23|%25|%20b.yaml` to Locations that keep the encoded filename, under both `/docs/:splat` and `/docs/*`. The browser kept `a%23b.yaml` in the pathname. The `?from=rule` and `/%72ender.yaml` rewrites served the identical 795-byte YAML with HEAD 200. All recorded controls held, all four surfaces and the fresh UI kept the ordered rows, and the typed-confirmation delete reached zero residue. Details: [live-acceptance.md](live-acceptance.md).
