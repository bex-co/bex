# w5 · m47 — Cron (and per-type) create discoverability: deep-linkable wizard + first-class New entries

**Worker:** worker5 **Goal:** Render's per-type create URLs (especially `/cron/new`) preselect the right service type, and each service type is a first-class "New" entry — so a user looking for cron finds it instead of landing on a Web-Service-defaulted wizard. **Status:** DONE 2026-07-18 — the create wizard preselects its type from `?type=`, `render-alias` maps every `/{web,worker,pserv,static,cron}/new` to `/services/new?type=…`, and both "New" menus surface a per-type service submenu (Cron Job included) — so `/cron/new` now lands on a cron-preselected form. Verified by typecheck + lint + the full 1591-test dashboard suite (new prefill/alias/create-context assertions); live browser proof was blocked by a pre-existing bex-api squatting the `local-bex` :8099 stub port, so automated coverage stands in.

## Tasks (in order)

| id   | title                                                                | est | depends_on       |
| ---- | -------------------------------------------------------------------- | --- | ---------------- |
| t001 | Wizard accepts + prefills `?type=` (deep-link to a type) — **DONE**  | 30m | —                |
| t002 | render-alias carries the service type on each create URL — **DONE**  | 20m | —                |
| t003 | First-class per-type "New" entries (at least Cron Job) — **DONE**    | 40m | t001, t002       |
| t004 | Render parity check — create-entry information architecture — **DONE** | 20m | t001, t002, t003 |
| t005 | Simplify the changed create-entry code — **DONE**                    | 20m | t004             |
| t006 | Test coverage — `?type=` prefill + alias mapping — **DONE**          | 30m | t004             |
| t007 | Closeout — **DONE**                                                  | 10m | t006             |

## Definition of done

- Navigating to `/services/new?type=cron_job` (or `/cron/new`, which the render-alias resolves) lands on the create wizard with the **Cron Job** type **already selected** and the schedule/command fields shown — not the Web-Service default. The same holds for `web`/`worker`/`pserv`/`static`.
- An unknown/absent `?type=` still defaults to `web_service` (no regression).
- At least one first-class **"Cron Job"** create entry exists in a "New" menu (dashboard-layout New-resource menu and/or the services-list New dropdown), linking to `/services/new?type=cron_job`, matching Render's distinct per-type create action.
- New labels are translated in both `en` and `zh`.
- Tests assert (a) the wizard preselecting `serviceType` from `search.type` including the unknown-value fallback, and (b) `render-alias` mapping each per-type create URL to the type-carrying `/services/new?type=…`.
- `yarn typecheck && yarn lint && yarn test` pass.

## Source + Goal linkage

- **Source:** User request 2026-07-18 ("hand off to /pm for cron jobs support across all places … but I don't see such thing in dashboard"). Root-caused to the create-entry flow: `common/lib/render-alias.ts:34-38` collapses all per-type create URLs (`/web|worker|pserv|static|cron/new`) to a bare `/services/new`; `parseNewServiceSearch` (`features/services/lib/create-context.ts`) accepts only `projectId`/`environmentId`; `routes/services.new.tsx:451` hard-initializes `serviceType` to `web_service`; the New menus (`common/components/dashboard-layout/new-resource-menu.tsx:29`, `routes/index.tsx:176`) offer only a generic "New service". The cron feature itself is already shipped + deployed on every surface (w1/m15 type + CronJob reconcile, w2/m36 run history; dashboard create-wizard Cron Job card, `cron-deploy-section.tsx`, `cron-runs-section.tsx`; documented in the new `docs/ADR038-cron-jobs.md`). This is purely the create-side follow-on to **w5/m39**, which handled the _detail_ deep-links (`/cron/$` → service detail) but left the _create_ deep-links type-less.
- **Goal linkage:** Render dashboard parity ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) — bex-api already accepts `type` at create on REST/GraphQL/MCP; this closes the one surface where cron creation is real-but-undiscoverable (the dashboard entry flow).
- **Expected outcome:** A user (or a Render-shaped deep link) reaching cron creation lands on a cron form, and "Cron Job" is a visible first-class create action — resolving the reported "I don't see cron in the dashboard".
- **Why now:** Directly triggered by the user's report; small, self-contained, and the last discoverability gap after the feature and its ADR are complete. **Render-parity task included** because this is a dashboard UI change (create-entry IA); note the backend surfaces already accept `type`, so parity scope is the dashboard entry flow only.
