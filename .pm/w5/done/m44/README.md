# w5 · m44 — Service Environment page parity

**Worker:** worker5 **Goal:** `/services/$serviceId/env` matches Render's safe, staged Environment workflow: read-only masked values until Edit; one coherent env-var/secret-file draft; copy/download export; `.env` paste/file import; previewable generated secrets; secret-file upload; Save only / Save and deploy / Save, rebuild, and deploy; responsive linked-environment-group UX. **Status:** DONE 2026-07-17 — full Core/API/dashboard/operator implementation, production-shaped desktop/mobile browser audit, and all repository gates passed.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture Render's complete live Environment UX and the bex desktop/mobile baseline — **DONE** | 45m | — |
| t002 | Batch environment patch + save-mode contract across Core, REST, GraphQL, and MCP — **DONE** | 1h | t001 |
| t003 | Staged Environment Variables view/edit editor with one discardable draft — **DONE** | 1h | t002 |
| t004 | `.env` paste/file import, previewable generated secrets, and copy/download export — **DONE** | 1h | t003 |
| t005 | Staged Secret Files editor with content dialog and multi-file upload — **DONE** | 1h | t002, t003 |
| t006 | Three save choices, one-rollout orchestration, dirty-state guard, and partial-failure UX — **DONE** | 1h | t004, t005 |
| t007 | Linked Environment Groups section + complete create/link path — **DONE** | 45m | t001 |
| t008 | Responsive/a11y/i18n polish and a trustworthy `local-bex` browser fixture — **DONE** | 1h | t006, t007 |
| t009 | Render parity check + durable evidence + ADR018 surface audit — **DONE** | 30m | t008 |
| t010 | Simplify the milestone's changed code — **DONE** | 30m | t009 |
| t011 | Test coverage for batch semantics, draft behavior, imports/uploads, and mobile layout — **DONE** | 1h | t010 |
| t012 | Closeout — **DONE** | 15m | t011 |

## Definition of done

On a live bex dashboard (dev-5 and the production-shaped target route), the Environment page opens in a read-only masked state and enters one staged editor only after Edit. Users can add/update/remove multiple variables and files, import dotenv text or a `.env` file, upload secret files, preview a generated value, copy or download a fail-closed dotenv export, discard the whole draft, and commit it with exactly one of three effects: store only, one rollout without rebuild, or one rebuild/deploy without an intermediate rollout. Unchanged masked values survive without being fetched or overwritten. Linked groups use the complete create/link experience and expose no workspace-destructive delete from the service page. Desktop and 390px mobile browser passes have no horizontal overflow; keyboard, focus, and screen-reader names work in English and Chinese. Backend and dashboard tests pass, `local-bex` serves warning-free environment fixtures, and `docs/render-artifacts/service-environment-page.md` plus ADR018 record shipped behavior and deliberate drift.

## Completion record

- **Shipped implementation:** one leak-disciplined mixed patch in Core with REST, GraphQL, and MCP adapters; pending first-projection annotations preserve true save-only semantics; old write routes remain immediate-roll compatible. The dashboard now has one masked view/edit draft for variables and files, import/export/generate/upload, three rollout choices, safe retry after deploy failure, navigation guards, and complete linked-group create/link UX.
- **Browser proof:** the real route against `local-bex` passed at 1440×900 and 390×844 with no page overflow. The scripted walk proved cancel-before-write, copy/download, generated/import/upload staging, one patch plus deploy-only retry after an injected trigger failure, save-only with no deploy, dirty-navigation confirmation, and populated group create/unlink/relink. Environment fixture warnings were zero; known global TanStack/jsdom warnings remain outside scope.
- **Repository gates:** `cd lego/backend && go test ./...`; `cd lego/operator && make test`; `cd dashboard && yarn typecheck && yarn lint && yarn test` (235 files / 1,484 tests after rebasing onto current `origin/main`). The operator's pre-existing asynchronous meter assertion was made deterministic without changing production behavior and passed 100 focused repetitions before the full suite.
- **Simplify review:** retained one typed draft/diff path, one save orchestrator, pure parser/diff helpers, and the shared complete group creator. No additional consolidation was accepted across opaque values, Secret projection, and rollout because those boundaries enforce preservation and zero/one-roll correctness.
- **Evidence:** `docs/render-artifacts/service-environment-page.md`, ADR006, ADR013, and ADR018 record the implemented contract and honest drift. The authenticated production bex route redirected this browser profile to login; no boundary was bypassed, and these uncommitted changes were not claimed as deployed.
- **Accepted drift:** Render's `Datastore URL` picker remains forbidden by `.pm/DO_NOT_DO.md`; environment-group writes retain their separately documented immediate-roll behavior.

## Source + Goal linkage

- **Source:** user request 2026-07-17 to learn the complete authenticated Render experience at `dashboard.render.com/web/srv-d1iai4be5dus739h1gmg/env`, apply it to `dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/env`, and arrange the work under w5. The live capture and bex comparison are in `docs/render-artifacts/service-environment-page.md`; t001 is already complete.
- **Goal linkage:** pillar 1 / Render compatibility (`docs/ADR008-vision.md`, `docs/ADR006-bex-api.md`). Environment configuration is a high-risk user-facing workflow: staged review, explicit rollout cost, and safe secret handling directly affect whether Render-trained users can operate bex without surprises.
- **Expected outcome:** one coherent Environment editor replaces immediate per-row pod rolls; multi-value setup becomes fast, Cancel becomes trustworthy, rollout intent is explicit, secret-file and linked-group flows match the rest of the dashboard, and the currently unusable narrow-screen form becomes operable.
- **Why now:** the live comparison found both capability drift and a severe 390px overflow on a core service page. The enabling APIs already exist (`setEnvVars`, secret-file CRUD, generated values, trigger deploy, env-group create/link), so the remaining backend seam is bounded: apply a staged patch while separating Secret projection from rollout and composing at most one deployment.
- **Standing closing tasks:** included as t009 Render parity, t010 Simplify, t011 Test coverage, and t012 Closeout because this changes REST/GraphQL/MCP semantics and the dashboard UI.
- **Explicit exclusion:** Render's newly observed `Datastore URL` picker remains excluded by `.pm/DO_NOT_DO.md` (copy/paste the datastore URL or use Blueprint `fromDatabase`). This milestone does not reopen that anti-goal, persist secrets in browser-readable storage, change local-value-over-group precedence, or add a hosted shell/job/disk capability.
