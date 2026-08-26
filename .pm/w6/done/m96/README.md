# w6 · m96 — Fix Render OpenAPI compat gate rejecting every REST Blueprint-id route (blueprintId pattern mismatch)

**Worker:** worker6 **Goal:** `GET/PATCH/DELETE /v1/blueprints/{id}` and `GET /v1/blueprints/{id}/syncs` return real data (or a real 403/404) for a caller using bex's own `blp-…` Blueprint id — the id shape bex actually mints and the only one that will ever exist — instead of unconditionally 400ing every request before authz or lookup ever runs. **Status:** done — three defects on these routes, not one; all fixed and gated green. Live REST verification carried to `w6/040` (blocked on the deploy pipeline)

## Background (found live, 2026-08-25/26 `/qa-find-bugs` hunt)

`lego/backend/internal/api/render_openapi.go` wraps every `/v1/*` REST request in `openapi3filter.ValidateRequest` against the embedded, pinned copy of Render's real public OpenAPI spec (`internal/api/openapi/render-public-api-1.json`). That spec pins the `blueprintId` path parameter to Render's own historical id shape: `pattern: "^exs-[0-9a-z]{20}$"`. bex mints Blueprint ids as `blp-…` (`lego/backend/internal/id/id.go:79`, `Blueprint = Kind{prefix: "blp", ...}`). No bex-minted Blueprint id has ever matched `exs-…`, and none ever will — so every REST call to a blueprint-id-scoped route is rejected by the validator **before** the request reaches `apps.Service`'s auth/lookup logic at all.

Live reproduction against `dashboard.bex.co`'s real backend (`api.bex.co`), from inside the authenticated dashboard page (`fetch(..., {credentials:'include'})`, not a bare script — see the hunt's Phase-3 trap about non-browser UAs):

```
GET https://api.bex.co/v1/blueprints/blp-d9nqg95cavls73fp8m10          -> 400 {"error":"invalid path parameter \"blueprintId\"","id":"bad_request","message":"invalid path parameter \"blueprintId\""}
GET https://api.bex.co/v1/blueprints/blp-d9nqg95cavls73fp8m10/syncs    -> 400 (identical body)
GET https://api.bex.co/v1/blueprints                                  -> 200 (list route has no path id, unaffected)
```

`blp-d9nqg95cavls73fp8m10` is a real, real, currently-`Error`-state blueprint (`discourse_docker`) owned by the QA workspace being used for this hunt — not a synthetic id. The dashboard itself never surfaces this because it drives blueprints exclusively over GraphQL (`SyncBlueprintDocument`, `DisconnectBlueprintDocument`, the `BlueprintSyncs`/`blueprint` queries in `dashboard/src/graphql/definitions.ts`) and MCP's `get_blueprint`/`sync_blueprint`/`update_blueprint`/`disconnect_blueprint`/`list_blueprint_syncs` tools (`internal/apps/mcp.go:344-407`) call the service layer directly — neither transport is wrapped by `render_openapi.go` (only the `/v1/` REST mux is, `internal/api/server.go:1022-1034`). So dashboard users are unaffected; **any REST-only client — the pattern ADR006/ADR020 exist specifically to serve — cannot ever successfully call these four operations.**

**Exhaustive blast-radius check (not an estimate):** cross-referencing `TestRenderRouteIntersectionInventory`'s full operation-id list (`internal/api/render_openapi_test.go:192`, every operation where a bex route matches Render's spec) against every `pattern`-constrained path parameter in the embedded spec found exactly **5** id-shaped path parameters with a hard regex, and checked each against the matching `id.Kind` prefix in `internal/id/id.go`:

| path parameter | Render pattern | bex prefix | match? |
| --- | --- | --- | --- | --- |
| `diskId` (`delete-disk`) | `^dsk-[0-9a-z]{20}$` | `dsk` | yes |
| `webhookId` (`delete-webhook`, `retrieve/update-webhook`, …) | `^whk-[0-9a-z]{20}$` | `whk` | yes |
| `eventId` (`retrieve-event`) | `^evt-[0-9a-z]{20}$` | `evt` | yes |
| `jobId` (`retrieve-job`) | `^job-[0-9a-z]{20}$` | `job` | yes |
| `blueprintId` (`retrieve/update/disconnect-blueprint`, `list-blueprint-syncs`) | `^exs-[0-9a-z]{20}$` | `blp` | **no** |

**Blueprint is the only mismatch in the entire currently-intersected REST surface.** The other four kinds' prefixes were evidently chosen to match Render's contract on purpose (ADR020's own aligned-prefix table lists `tea-/srv-/dpg-/red-/cdm-/evt-/crr-` "Render's public-API spellings, so bex ids are drop-in for Render-shaped clients" — `docs/ADR020-identifiers.md:14-24`); Blueprint was minted with bex's own mnemonic prefix instead (`w2/m15`, predates the OpenAPI compat gate added later in `a2dda880`), and ADR020's "Known deviations (deliberate, documented)" section never records it — this was never a deliberate, documented divergence, it is an untested gap. `w2/m62`'s own Definition of done states "`GET/PATCH/DELETE /v1/blueprints/{id}` and `GET /v1/blueprints/{id}/syncs` behave per Render's OpenAPI" and was marked done — but the only REST test for these routes (`TestRESTListBlueprintSyncsIncludesErrorMessage`, `internal/apps/blueprint_test.go:1365`) builds a **bare `http.NewServeMux()`** and calls `svc.RegisterREST(mux)` directly, bypassing `render_openapi.go` entirely. No test anywhere in the repo exercises a blueprint-id route through the actual composed server (the one with the validator wired in), so this was never actually verified and CI has never caught it.

**Adjacent classes collapse:** because the validator runs after authentication but before any service-level authorize/lookup (`internal/api/server.go:1034`, `TestRenderValidationRunsAfterAuthentication`), every call — a real id in your own workspace, a real id in someone else's workspace, or a nonexistent id — returns the identical generic `400 bad_request`. A correctly-scoped fix must let the normal `AuthorizeApp`-style flow resume distinguishing 200 / 403 / 404 for these routes; it must not simply swap one blanket status for another.

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Compatibility fix: stop rejecting bex's own `blp-…` Blueprint id at the Render OpenAPI gate | 30m | — | — **DONE** |
| t002 | Regression tests through the real composed server (not the bare-mux shortcut) for all 4 blueprint-id routes + a table test locking in the other 4 already-correct id/pattern pairs | 40m | t001 | — **DONE** |
| t003 | Correct ADR020 (record the deviation) and ADR018's Blueprint parity row | 15m | t001 | — **DONE** |
| t004 | Render parity | 20m | t002, t003 | — **DONE** |
| t005 | Simplify | 15m | t004 | — **DONE** |
| t006 | Test coverage | 20m | t005 | — **DONE** |
| t007 | Closeout | 10m | t006 | — **DONE** |

## Definition of done

- `curl`/`fetch`-equivalent `GET https://api.bex.co/v1/blueprints/{a real blp-… id in the caller's own workspace}` returns 200 with the blueprint body (not 400), live-verifiable on `dashboard.bex.co`'s backend once deployed.
- The same for `PATCH`/`DELETE /v1/blueprints/{id}` and `GET /v1/blueprints/{id}/syncs` — all four return their real REST-documented response, never the generic `invalid path parameter "blueprintId"` 400, for any syntactically-valid `blp-…` id.
- A `blp-…` id that does not exist, or belongs to a workspace the caller cannot access, now reaches the service layer and gets the **correct** distinct outcome (404 / 403) instead of being indistinguishable 400s — asserted by a new test, not just implied.
- A new test asserts this through the **actual composed server** (auth + rate-limit + `renderRequestValidator`, not a bare `http.NewServeMux()` shortcut) — closing the coverage gap that let this ship as "done" in `w2/m62` without ever being true.
- A table test enumerates every currently pattern-constrained id path parameter in the Render-intersected REST surface (`diskId`, `webhookId`, `eventId`, `jobId`, `blueprintId`) against its `id.Kind` prefix, so a future spec refresh silently reintroducing a mismatch (on any of the five, not just Blueprint) fails CI instead of shipping quietly.
- GraphQL (`blueprint`, `blueprintSyncs`, `syncBlueprint`, `updateBlueprint`, `disconnectBlueprint`) and MCP (`get_blueprint`, `sync_blueprint`, `update_blueprint`, `disconnect_blueprint`, `list_blueprint_syncs`) are reconfirmed unaffected (they already work; this task must not touch their behavior) — Render parity task records this explicitly rather than assuming it.
- ADR020 gains either a corrected aligned-prefix entry or a documented deviation for Blueprint (and its sync id `bsr-` vs Render's `exe-`, which shares the same class of gap but was not found to be pattern-constrained in the current spec — recorded as unverified, not asserted safe).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 2026-08-25/26 (4th run of the day, run from a scheduled cron session). Found while investigating the Blueprints surface's real production data (`discourse_docker`, `blp-d9nqg95cavls73fp8m10`, workspace `tea-d98210cbbpdc73dcrkvg`) after ruling out the dashboard's already-known/already-fixed-but-undeployed sync-error-message gap (`w6/m50`, confirmed fixed on `main` post-`717143ae`, not yet deployed — see `w6/040`) as the explanation for what a raw `fetch` from inside the page actually returned. Evidence: the exact request/response pairs quoted above, `.playwright-mcp/qa-blueprints-1.png` (the real blueprint's Error-state detail page), captured 2026-08-26.
- **Goal linkage:** Render REST compatibility is a named product pillar (`docs/ADR006-bex-api.md`, `docs/ADR020-identifiers.md` — "bex ids are drop-in for Render-shaped clients") and was the explicit, already-closed Definition of done of `w2/m62` ("behave per Render's OpenAPI"). This milestone corrects that DoD, which shipped without the coverage to actually prove it.
- **Expected outcome:** a Render-API-compatible REST client (the class of caller ADR006/ADR020 exist to serve — third-party tooling, Terraform-style providers, curl/scripts) can manage a bex Blueprint by id over REST at all, for the first time.
- **Why now:** found live, currently reproducible on production (unaffected by the ongoing `deploy.yml` freeze tracked in `w6/040` — `render_openapi.go` hasn't changed since before `717143ae`, so this is a live bug on `main` too, not deploy lag), affects a shipped-and-closed milestone's own stated guarantee, and the fix is narrowly scoped (confirmed by exhaustive grep to be the only mismatch in the intersected surface) so it is cheap to close correctly now before the surface grows.
- **Render parity task included:** yes (t004) — this is a fix to a REST/GraphQL/MCP-exposed surface; GraphQL/MCP are already correct and t004 reconfirms rather than assumes that.
