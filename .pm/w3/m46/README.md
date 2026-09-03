# w3 · m46 — Static-site Render-parity fixes (suspend page · delete-row projection · clear-cache deploy · SPA-fallback reconciliation)

**Worker:** worker3 **Goal:** close the four static-site behavioral inconsistencies vs Render surfaced by the 2026-08-21 live parity walk, so a bex `static_site` matches Render (or diverges only by documented, deliberate design). **Status:** implementation complete + locally verified (t001–t007 done); t008 awaits live closeout of the original four behaviors. The later deleting-App by-id/finalizer defect outgrew a task and was promoted to [m81](../m81/README.md) instead of distorting this milestone's finished closing-task chain.

## Tasks (in order)

| id   | title                                                                    | est | depends_on                   |
| ---- | ------------------------------------------------------------------------ | --- | ---------------------------- |
| t001 | Suspended static site: serve a suspended page on the managed cert / fix copy | 1h  | —                            — **DONE** |
| t002 | Deleted static site lingers as "Unknown" in the resource list — fix projection | 45m | —                            — **DONE** |
| t003 | Wire `clearCache` into Manual Deploy menu + MCP `trigger_deploy`         | 45m | —                            — **DONE** |
| t004 | Reconcile the default SPA-fallback divergence (decide + document + optional gate) | 45m | —                            — **DONE** |
| t005 | Render parity sweep across REST/GraphQL/MCP/UI                           | 30m | t001, t002, t003, t004       — **DONE** |
| t006 | Simplify the changed code                                                | 20m | t005                         — **DONE** |
| t007 | Test coverage for the shipped behavior                                   | 40m | t005                         — **DONE** |
| t008 | Closeout                                                                 | 10m | t007                         |

## Implementation summary (t001–t007 done 2026-08-21, landed in `167eecf5`)

- **t001 (suspend cert):** the operator no longer removes a static site's Ingress on suspend — it keeps the host Ingress + TLS cert pointed at the static-server (`reconcileStaticIngress`, `app_controller.go`); the resolver already drops suspended sites, so the static-server serves its ordinary 404 over the **managed** cert instead of Traefik's default self-signed cert. The dashboard "URL and certificates are kept" copy is now truthful. Envtest `keeps the host Ingress + certificate on the static-server when suspended` (5.3s, passing). Documented in ADR029 § Suspend/resume.
- **t002 (lingering delete row):** `apps.Service.List` now excludes Apps with a `DeletionTimestamp`, so a deleted static site leaves the Overview list at once (matching Render + the already-404 by-id Get) instead of lingering as "Unknown". Test `TestListOmitsDeletingApp`.
- **t003 (clearCache):** `ClearCache` added to the shared `TriggerParams` + `validateTrigger` (enum-validated no-op — bex builds are cache-free) and threaded through REST/GraphQL/**MCP** + the dashboard Manual Deploy menu ("Clear build cache & deploy"). `trigger_deploy` moved from MCP `Divergent` → `Superset`; `mcpAcceptedDivergences` entry deleted; inventory guard + ADR018/ADR006 updated. Tests: `TestTriggerRejects/AcceptsClearCache*`, `TestRESTTriggerClearCache`, `manual-deploy-button.test.tsx`, MCP parity guards.
- **t004 (SPA fallback):** decided **keep-by-design** (extension-less miss → root `index.html`; asset miss → 404) and documented the deliberate Render divergence in ADR029 § Default SPA fallback + the ADR018 static-site row. Behavior already covered by `TestImplicitSPAFallback`.
- **t005/t006/t007:** parity swept across all four surfaces (docs updated); `/simplify` extracted `reconcileStaticIngress` (shared by the running + suspend paths) and trimmed a redundant comment; tests added/confirmed as above. Local runs: backend `go test ./...` (59 ok), operator static-site envtests, dashboard `manual-deploy-button` (4/4) — full dashboard suite is 2284/2284 loadable tests passing (10 files fail to load only on a missing `@tanstack/react-virtual` dep, unrelated).

**t008 gate:** deploy the operator + bex-api + dashboard and re-verify the original four-behavior scope on a live `onbex.co` static site (suspend → managed-cert 404, delete → row gone, clear-cache menu deploy, documented SPA fallback). The separately discovered deleting-App by-id/finalizer contract is tracked in m81 and is not silently folded into this milestone after its parity/simplify/test tasks already ran.

## Definition of done

Each of the four inconsistencies is resolved with an observable end state, verified against the live `onbex.co` edge where applicable:

1. A **suspended** static site no longer presents Traefik's default self-signed cert with a bare `404 page not found`: either it serves a branded suspended page over the valid managed (Let's Encrypt) cert, or — if the maintenance-page route is out of scope — the dashboard Settings copy no longer claims "Its URL and certificates are kept" while suspended. Resume restores normal serving (regression-tested).
2. **Deleting** a static site removes its row from the Overview / resource list promptly, with no lingering `Unknown` row in the list. Direct by-id visibility while finalization is still running is the broader lifecycle contract tracked in m81.
3. Render's **Clear build cache & deploy** (`clearCache`) is reachable from the dashboard Manual Deploy menu **and** MCP `trigger_deploy` (closing the adapter gap already noted at `lego/backend/internal/api/mcp_parity.go:120`), matching REST/GraphQL.
4. The default **SPA fallback** (extension-less miss → `/index.html`) divergence from Render's default-404 is an explicit, documented decision in `docs/ADR029-static-sites.md` + the ADR018 parity ledger — kept by design, matched to Render, or made configurable — not an undocumented drift.

## Source + Goal linkage

- **Source:** the 2026-08-21 live Playwright static-site parity walk on `dashboard.bex.co` (created → deployed → routes → headers → manual redeploy → suspend/resume → delete), which surfaced four bex↔Render inconsistencies. User directed `/pm for fix inconsistency for w3` and confirmed "all" four in scope.
- **Goal linkage:** the Render-alternative core — Render API/UX compatibility. Advances the `docs/ADR018-render-parity.md` ledger and `docs/ADR029-static-sites.md` (the `static_site` type across REST/GraphQL/MCP/UI).
- **Expected outcome:** a bex `static_site` behaves like Render across suspend, delete-from-list, and clear-cache deploy; the one deliberate divergence (default SPA fallback) is documented rather than surprising.
- **Why now:** found live on production. The suspend behavior is a real correctness/trust defect — a suspended site breaks TLS for visitors (browser cert error) while the UI copy promises the cert is kept — and the lingering "Unknown" delete row misrepresents the resource list. Both are user-facing and cheap to fix while the evidence is fresh.
- **Render parity task included** because the milestone changes user/tenant-facing surfaces (dashboard UI + MCP `trigger_deploy` + static-site serving/lifecycle) that must stay consistent with Render across REST/GraphQL/MCP/UI.
