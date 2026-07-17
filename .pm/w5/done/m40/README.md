# w5 · m40 — Deploys parity polish: searchable rich history + actionable diagnostics

**Worker:** worker5 **Goal:** close the verified presentation and diagnostic gaps between bex's Web Service Deploys pages and Render's corresponding Web Service pages without reopening already-shipped deploy mechanics or scheduling product non-goals. **Status:** done (2026-07-16)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Rich deploy-history rows: search, honest count, duration, metadata, eligible rollback actions — **DONE** | 60m | — |
| t002 | Deploy-detail diagnostic polish: metadata hierarchy + truthful log states — **DONE** | 45m | t001 |
| t003 | Web-service header parity: runtime fact + coherent service/deploy status — **DONE** | 45m | t002 |
| t004 | Live dev-5 acceptance: failed and successful deploy paths against the corrected Render target — **DONE** | 45m | t001, t002, t003 |
| t005 | Render parity — re-walk list, detail, header, and actions — **DONE** | 30m | t004 |
| t006 | Simplify — `/simplify` over the milestone's diff — **DONE** | 20m | t005 |
| t007 | Test coverage — meaningful tests for the shipped behavior — **DONE** | 45m | t005 |
| t008 | Closeout — move to `done/` when the DoD holds — **DONE** | 15m | t007 |

## Definition of done

- The Deploys page supports text search across the loaded deploy id, commit SHA, and commit message while preserving status filtering and pagination; it labels its count honestly when more pages remain.
- Each deploy row presents the available status, commit, trigger, deployed time, and duration in a scannable hierarchy. The existing shared rollback action appears only when the deploy is actually eligible; no new rollback backend is invented.
- The deploy detail page makes the same available metadata easy to scan and keeps successful-empty, log-store-unavailable, and disconnected/live-tail states distinct. It never fabricates a failure reason or claims build logs exist when `BEX_LOKI_URL` is absent.
- The Web Service header exposes the already-queried runtime when present and does not present a terminal failed latest deploy as though that deploy were still building. Service phase and deploy status remain separately named facts when they differ.
- Browser acceptance covers one failed deploy and one successful/live deploy on dev-5, including search/filter behavior, successful-history rollback eligibility, responsive layout, and the truthful no-log-store path.
- Dashboard tests and locale coverage pass; parity evidence records any residual divergence rather than silently accepting it.

## Source + Goal linkage

- **Source:** user-requested Playwright comparison on 2026-07-16 between `http://localhost:7005/services/node-hello/deploys/dep-d9cp31a9086lu3qou6a0` and the corrected Render Web Service target `https://dashboard.render.com/web/srv-d2rnr3jipnbc73deuvgg?`. Captures are in the gitignored `.playwright-mcp/` directory.
- **Observed gap:** Render's history is searchable and count-bearing, with denser status/commit/trigger/date/duration rows and eligible rollback actions; its deploy detail and service banner make runtime, commit, live state, and log-availability context easier to distinguish. The dev-5 comparison also showed a service-level `Building` badge beside a terminal `Build Failed` deploy.
- **Goal linkage:** `docs/ADR008-vision.md`'s human-facing Render alternative and `docs/ADR018-render-parity.md`'s dashboard surface; this milestone polishes the existing deploy API/UI contract rather than adding a parallel surface.
- **Expected outcome:** operators can find a deploy quickly, understand what happened from one row/detail view, and trust the distinction between service phase, deploy result, and log-store availability.
- **Why now:** w5/m21 and w5/m29 already shipped deploy actions/detail, w2/m38 shipped the lifecycle vocabulary, and w7/m28 shipped Loki-backed build logs. The live comparison therefore isolates a bounded dashboard polish pass instead of another cross-layer deploy build.
- **Render parity:** included (t005) — this is UI-surface parity work.

## Explicit exclusions and existing owners

- Persistent disks, browser Shell, one-off jobs, and PR previews are hard non-goals in `.pm/DO_NOT_DO.md`; they are not scheduled here even though Render's navigation exposes them.
- Running-instance SSH is owned by w2/m39. Commit-author timestamp was owned by w2/m42 and shipped concurrently before m40's final rebase.
- Cancel/Rollback mechanics and deploy detail/navigation already shipped in w5/m21, w5/m29, and w9/m1; tasks here only reuse or polish those capabilities.
- Loki build-log ingestion shipped in w7/m28. Configuring an external log drain or pretending a storeless dev stack has retained build logs is out of scope.

## Closeout notes (2026-07-16)

- **Shipped:** Deploy history now searches loaded ids/commits/messages, distinguishes loaded counts from complete totals, presents shared timestamp/duration metadata, and exposes the existing rollback confirmation only on successful history. Deploy detail distinguishes query errors, successful-empty results, unavailable durable history, partial application logs, and live-tail disconnection. The service header names service phase and latest-deploy result separately and shows the supplied runtime.
- **Live acceptance:** Playwright exercised failed `dep-d9cp31a9086lu3qou6a0`, live `dep-d9cplqi9086lu3qou6eg`, and historical deactivated `dep-d9cplna9086lu3qou6dg` on dev-5, including composed search/filter behavior, rollback confirmation without mutation, truthful storeless logs, desktop layout, and keyboard focus at `390x844`. The authenticated Render reference was re-walked without mutation. Screenshot names and the complete residual classification are recorded in `docs/render-artifacts/deploy-detail-page.md`.
- **Parity:** `docs/ADR018-render-parity.md` retains the four-surface ✅ and now links the 2026-07-16 UI proof. The final rebase also incorporated w2/m42's shipped commit-author timestamp. Remaining residuals are explicitly classified: running-instance SSH → `w2/m39`; missing fixture commit/log history → environment/data; internal address → deliberate presentation-scope boundary; Shell/disks/jobs/previews/drains → hard non-goals.
- **Simplify:** list, detail header, and timeline share one timestamp helper; list/detail share one duration helper and one status vocabulary; cancel/rollback rules remain in the shared action component while list-only historical eligibility prevents a redundant current-live rollback. Distinct log states deliberately remain separate branches because collapsing them would erase diagnostic meaning.
- **Quality gates:** dashboard `yarn lint`, `yarn typecheck`, and the focused deploy/service suites passed. Full `yarn test` passed 216 files / 1,270 tests. The final browser acceptance passed after the historical-only action correction.
