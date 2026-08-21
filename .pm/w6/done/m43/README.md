# w6 · m43 — New-Service and status-badge copy bugs for non-HTTP service types

**Worker:** worker6
**Goal:** Close three reproducible dashboard divergences a live bex-vs-Render Background Worker lifecycle QA pass surfaced today — each is the dashboard showing web-service-shaped copy or status to a service type (`background_worker`, and in one case `cron_job`) that structurally cannot have what the copy describes.
**Status:** done

## Tasks (in order)

| id   | title                                                                          | est | depends_on              |
| ---- | ------------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Gate the Existing-Image `$PORT` hint on service types that actually have a port — **DONE** | 30m | — |
| t002 | Stop deriving "sleeping" status for a service type that can never auto-sleep — **DONE** | 40m | — |
| t003 | Make the New Service page title/subtitle reflect the selected service type — **DONE** | 30m | — |
| t004 | Render parity — **DONE** | 20m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- The Existing-Image tab's `$PORT`/privileged-port helper text renders only for service types that actually bind an HTTP port (`web_service`, `private_service`); it no longer appears for `background_worker` (verified: it currently renders unconditionally whenever an image source is offered, contradicting the "This service type has no public URL." note on the same page).
- Suspending then resuming a `background_worker` (or any type ineligible for free-tier auto-sleep) never shows the **Sleeping** badge or "wakes on the next request" copy during the resume transition — `deriveStatus` maps phase `Hibernated` to that key only for types that can actually auto-sleep.
- The New Service page's `<h1>`/subtitle pair reflects the selected `Service Type` for all five types (or reads a generic, type-neutral sentence) — today only `cron_job` gets its own copy; `background_worker`/`private_service`/`static_site` silently fall through to the `web_service` strings.
- Each fix has a test that fails on the pre-fix behavior and passes after.

## Source + Goal linkage

- **Source:** live QA walkthrough of the Background Worker lifecycle against a real Render account and hosted bex.co, 2026-08-21 — full report: https://claude.ai/code/artifact/29934e6e-7bd8-4fa0-8a86-26327bb72932. Findings 1, 2, 4 from that report (findings 3 and 5 need a product decision first — filed separately as `w6/025.md` and `w6/026.md`).
- **Goal linkage:** [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md)'s Render-compatibility mandate and [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md), which already marks Background Worker ✅ across REST/GraphQL/MCP/**UI** (`docs/ADR018-render-parity.md:71`) — these three bugs are in the dashboard UI cell of that same row, so the ledger currently overstates a claim the running product doesn't back up.
- **Expected outcome:** the New Service wizard and service status badge stop showing HTTP/free-tier-sleep language to service types that structurally cannot have either — the exact surfaces a new Background Worker user hits first (the create form's image-source step) and mid-incident (reading a suspend/resume transition).
- **Why now:** all three are small, isolated, precisely located dashboard fixes (confirmed against source, not guessed) surfaced by a live comparison against the real Render product done today; cheap to fix now versus compounding into more copy that assumes every service has a port and can sleep.

## Outcome (2026-08-21)

All three findings fixed, each pinned by a test proven red against the pre-fix
tree (7 failures in `services.new.test.tsx`, 1 in `status.test.ts`) and green
after. Verified live on `dev-6` in a browser as well as in unit tests, since
the findings themselves came from a live walkthrough:

- **t001** — the `$PORT` hint is now gated by `showPortHint`, a required field
  on `ImageSourceOption` so a future caller cannot inherit a wrong default.
  Confirmed live: absent for Background Worker, still present for Web Service.
- **t002** — `deriveStatus` resolves phase `Hibernated` by type. Only a type
  that serves HTTP can be a sleeper; anything else reports `pending`, the state
  a resuming worker is actually in.
- **t003** — `SERVICE_TYPE_CREATE_COPY` gives all five types their own heading
  and subtitle; the route's document title reads the same resolver.

**The `/simplify` pass (t005) found a real bug this milestone had introduced**,
which is why t003's fix is larger than its task file describes. `serviceTypeCreateCopy`
originally fell back to generic copy for an absent `?type=`, while the form
defaulted to `web_service` — so a bare `/services/new` showed "New Service" in
the tab and "New Web Service" in the `<h1>`, the exact drift t003 set out to
remove. Fixed by exporting `DEFAULT_SERVICE_TYPE` and resolving both paths
through it; `route-heads.test.ts` had been asserting the drifted title and was
corrected, and `create-context.test.ts` now pins the two paths together.

Other `/simplify` outcomes:

- `servesHttp()` moved from `create-context.ts` to `service-type.ts`, which
  already held the predicate family (`isCron`/`isWorker`/`isWebService`/…). The
  original placement made `status.ts` import from the create wizard.
  `supportsMaxShutdownDelay` now composes on it instead of re-listing types.
- The `SLEEPING`/`RESUMING` module constants were removed — `RESUMING` was a
  literal duplicate of `PHASE_STATUS.pending` — restoring one phase table.
- `services.createDescription` deleted: byte-identical to the new
  `createWebDescription` in both locales and left with no callers.
- Efficiency review found nothing to change (the added branch is one string
  compare on a path already dominated by `toLowerCase()`).

**Deliberately not done** — `routes/services.$serviceId.settings.tsx:86,327`
spell `!cron && !worker && !staticSite`, which is `servesHttp` by exclusion and
was flagged as a reuse win. Converting it is **not** behavior-preserving: with
`service` still loading, all three booleans are `false` so the health-check
section currently renders, whereas `servesHttp` would hide it. That is a real
behavior question in a feature this milestone does not otherwise touch, so it
is filed rather than bundled into a copy fix — see `w6/027.md`.
