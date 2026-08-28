# w6 · m115 — Four read surfaces disagree about what a service's `name` is, and the one REST returns cannot address the service

**Worker:** worker6 **Goal:** one settled answer to "what is a service's `name`?" that REST, GraphQL, MCP and webhooks all give — with the immutable workspace-unique name still reachable, and every name a read surface returns still usable to address the service **Status:** done

**Settled contract (t001, → `docs/ADR006-bex-api.md` § The settled name contract):** `name` = Render's **mutable** `service.name` (the display label when set, else the immutable name), reported identically by REST/GraphQL/MCP and by the webhook `serviceName`. The immutable, workspace-unique, addressable name is exposed as **`immutableName`** on every read surface. Addressing (a bex extension — Render addresses by `id` alone) resolves `id` or the immutable name; the display label is deliberately **not** a resolution key.

**Implemented:**

- **t002 — the one-line divergence:** GraphQL `Service.name` now reads `renderServiceName(a)` (the shared helper REST/MCP/webhooks already use) instead of the raw immutable name, so the four read surfaces agree (`apps/graphql.go`).
- **t003 — immutable name has a home:** new `immutableName` field on the shared `renderService` struct (REST + MCP) and the GraphQL `Service` type, carrying `a.Name` — distinct from `slug` (which a cross-tenant collision can suffix). The webhook payload stays intentionally thin (its `serviceId` is the addressable identifier and it already agrees on `serviceName` = label); the immutable name is read by following `serviceId` back to the service object, matching the webhook's documented "fetch details via the API" design.
- **t004 — addressing, the safe alternative the DoD offers:** `displayName` is confirmed **unvalidated and non-unique** (`SetDisplayName` only trims), so making it a `core.Base.GetApp` resolution key would add an ambiguous, unbounded key to the function that gates every App-by-name authorization, plus a display-name existence oracle. Chose the milestone's stated alternative: addressing stays `id` + immutable-name (Render-consistent), and t003's `immutableName` field keeps the address recoverable from any read response. `GET /v1/services?name=` still matches either spelling (unchanged, guarded by a test).
- **t005 — parity:** `docs/ADR006` + `docs/ADR018` rename row updated with the settled contract, `immutableName`, and the addressing scope.
- **t007 — coverage:** `apps/name_contract_test.go` asserts REST/GraphQL/MCP return the same `name` (= label) and same `immutableName` (= immutable) for a renamed service **of all five App-CR-backed types**, that the immutable name resolves through `GetApp` while the display label does not, and that the list filter still matches both spellings; `displayname_test.go` updated to the settled contract.

**Conscious deviation from a literal DoD bullet:** bullet 3 ("every `name` a read surface returns addresses the service") is met via the DoD's own offered alternative rather than by making the display label addressable — the label is unvalidated/non-unique and admitting it into the authorization-gating lookup is unsound; `immutableName`/`id` in the same payload provide the recoverable address. Recorded in ADR006.

## Tasks (in order)

| id   | title                                                                                      | est | depends_on | status   |
| ---- | ------------------------------------------------------------------------------------------ | --- | ---------- | -------- |
| t001 | Settle the contract: what `name`, `slug` and `displayName` each mean on a read surface       | 30m | —          | **DONE** |
| t002 | Make GraphQL `Service.name` carry the settled label via the one shared helper                | 30m | t001       | **DONE** |
| t003 | Give the immutable workspace-unique name a home on every read surface                        | 45m | t001       | **DONE** |
| t004 | Make name-based addressing accept every name a read surface hands back — or stop handing it  | 45m | t002, t003 | **DONE** |
| t005 | Render parity                                                                                | 20m | t004       | **DONE** |
| t006 | Simplify                                                                                     | 20m | t005       | **DONE** |
| t007 | Test coverage                                                                                | 30m | t005       | **DONE** |
| t008 | Closeout                                                                                     | 10m | t006, t007 | **DONE** |

## Definition of done

Every bullet is a probe run against a service that has a `displayName` differing from its immutable name (create one, `PATCH {"displayName":"…"}`, then read).

- **The four read surfaces return the same string for `name`.** For the same service id, `GET /v1/services/{id}` `.name`, GraphQL `server(id:){name}`, MCP `get_service` `.name`, and the `serviceName` on the next webhook delivery are byte-identical. Today they are not: REST/MCP/webhooks return the display label and GraphQL returns the immutable name (capture below). Stating **which** string is correct is `t001`'s output and must be written into `docs/ADR006-bex-api.md` — "they agree" alone is satisfiable by normalizing every surface onto the value that cannot address the resource.
- **The immutable workspace-unique name is still readable from every surface**, under a field whose name says so. `slug` does **not** satisfy this: `AppView.Slug` is `a.Spec.PlatformSubdomain(a.Name)` (`apps/service.go:846`), documented at `service.go:346-353` as platform-unique and equal to `<name>-<4-char suffix>` after a cross-tenant collision — so on a collided service `slug != name` and reading `slug` yields a string `GetApp` cannot resolve.
- **Every `name` a read surface returns addresses the service.** `GET /v1/services/{that exact string}` returns 200, and GraphQL `server(id: "{that exact string}")` resolves. Today `GET /v1/services/eden-dash-v3` → `404 app not found` on the value REST itself just returned as `name`.
- **`GET /v1/services?name=<either name>` keeps matching both spellings.** `apps/rest.go:663` already accepts `a.Name` or `renderServiceName(a)`; whatever t004 decides must not narrow it.
- `go test ./lego/backend/internal/apps/... ./lego/backend/internal/store/...` includes a renamed-service case asserting REST, GraphQL and MCP return the same `name`, and that the immutable name is still retrievable and still resolves through `core.Base.GetApp`.
- The five App-CR-backed types are checked, not assumed: `web_service`, `private_service`, `background_worker`, `cron_job`, `static_site` all share `AppView`, so one probe per type confirms the shared path rather than sampling one.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 39th run, 2026-08-27, found while checking the notification page's per-service override dropdown against the service list. The dropdown listed `eden-dash-v3` and `qa-20260826-webhook-renamed`; the GraphQL `services` query for the same workspace listed the same two ids as `block-eden-mono` and `qa-20260826-webhook-svc`.

  Probe 1 — the same two ids across all four read paths, one instant, `page.evaluate` + `fetch(..., {credentials:'include'})` from `https://dashboard.bex.co`, workspace `tea-d98210cbbpdc73dcrkvg` (`2026-08-27T11:04:11.747Z`):

  ```
  srv-d9ndt8hmcglc739fkp50
    REST  {name:"eden-dash-v3",                slug:"block-eden-mono",           displayName:"eden-dash-v3"}
    GQL   {name:"block-eden-mono",             slug:"block-eden-mono",           displayName:"eden-dash-v3"}
  srv-da7o6ovvqdcc73bpn9hg
    REST  {name:"qa-20260826-webhook-renamed", slug:"qa-20260826-webhook-svc",   displayName:"qa-20260826-webhook-renamed"}
    GQL   {name:"qa-20260826-webhook-svc",     slug:"qa-20260826-webhook-svc",   displayName:"qa-20260826-webhook-renamed"}
  srv-d9bkcspg9s7c73d0n8ug   (control — no displayName set; all four agree)
    REST  {name:"agentmarketcap-1", slug:"agentmarketcap-1", displayName:""}
    GQL   {name:"agentmarketcap-1", slug:"agentmarketcap-1", displayName:""}
  ```

  The control is load-bearing: with `displayName` empty the surfaces agree, so the divergence is the display label specifically, not a caching or projection lag.

  Probe 2 — round-trip, same session (`2026-08-27T11:06:56.293Z`):

  ```
  GET /v1/services/eden-dash-v3        -> 404 {"error":"app not found","id":"not_found"}   <- the value REST returns as .name
  GET /v1/services/block-eden-mono     -> 200 name=eden-dash-v3                            <- the value REST does NOT return
  GQL server(id:"eden-dash-v3")        -> {"data":{"server":null},"errors":[{"message":"app not found"}]}
  GQL server(id:"block-eden-mono")     -> {"name":"block-eden-mono","slug":"block-eden-mono","displayName":"eden-dash-v3"}
  ```

  So each surface is internally coherent and they disagree: GraphQL's `name` round-trips and is stale; REST's `name` is current and 404s. That is why "make GraphQL match REST" is not the spec — it would import REST's 404 into the one surface that does not have it.

- **Root cause:** `lego/backend/internal/apps/graphql.go:299` — `"name": gqlutil.StrField(func(a AppView) any { return a.Name })` — reads the raw field, while REST at `render.go:254` reads `renderServiceName(a)`, whose own doc comment (`render.go:508-510`) states the intent: "maps bex's immutable public name + mutable display label onto Render's contract, **where service.name itself is mutable**." MCP is not a third implementation — `listServicesResult` (`mcp.go:50-52`) and `renderServiceResult` serialize the same `renderService` struct, so MCP inherits REST's behavior. Webhooks got the SQL spelling of the same helper in `w6/m101` (`store/webhooks.go:679`, `appDisplayLabel`). GraphQL is the one surface that never calls it. The 404 half has a separate cause: `core.Base.GetApp` (`core/base.go:1453-1495`) resolves `LabelAppID` then `LabelServiceName`, and `LabelServiceName` carries the immutable public name (`core/base.go:175-182`) — `displayName` is not a resolution key anywhere.
- **Goal linkage:** [docs/ADR006-bex-api.md](../../docs/ADR006-bex-api.md) — REST/GraphQL/MCP are one contract in three fragments. `lego/backend/internal/apps/mcp.go:36-39` states the invariant this breaks in so many words: "Every tool delegates to the same Service method REST/GraphQL call, **so the three surfaces cannot drift**." They have drifted, because the drift is in the serializer, not the service method. Also [ADR018](../../docs/ADR018-render-parity.md)'s rename row and [ADR020](../../docs/ADR020-identifiers.md) (what addresses a resource).
- **Expected outcome:** a third-party GraphQL client sees a renamed service's current name instead of its creation-time name, and any `name` an API response hands back can be fed straight back into a by-name read without a 404.
- **Why now:** `w6/m101` fixed this class on the webhook surface three days ago and its Goal line asserts the label matches "the dashboard/REST/**GraphQL** already show". Its own `## Source` section contains the counter-evidence verbatim — the GraphQL probe returning `{"name":"block-eden-mono","displayName":"eden-dash-v3"}` — and `m101/t004`'s Result calls GraphQL's `name` "(immutable id)" and passes it. The check t004 actually ran was **no-regression** ("byte-identical to before this milestone"), not cross-surface agreement, so the drift survived a parity task written to catch exactly this. `5067bd4d feat: add mutable service display names` introduced the split and `w8/m8`'s DoD proved each surface's `displayName` field individually; nothing has ever compared REST's `name` against GraphQL's `name`. Leaving it means the next surface to grow a service label copies whichever neighbour it happens to read.
- **Render parity:** included (t005) — every affected field is on a Render-compatible read surface. Render has no mutable/immutable split: its `service.name` is mutable and its `GET /v1/services/{serviceId}` accepts **only** the `srv-…` id, never a name. So Render's contract answers t001 for `name` and makes bex's name-addressing a bex extension whose scope t004 must state.
- **Blast radius:** the label half is exactly one line (`graphql.go:299`); `renderServiceName` has exactly two non-test callers (`render.go:254`, `rest.go:663` — verified by grep). All five App-CR-backed types share `AppView`, so all five move together. Postgres and Key Value are unaffected because they have no resource display name — re-verified this run rather than inherited from m101: `grep -rn "DisplayName\|displayName" lego/backend/internal/postgres/ lego/backend/internal/keyvalue/ | grep -v _test` returns 7 lines, and all 7 are `pgTierDisplayName`/`kvTierDisplayName` plan-tier label helpers (`postgres/tiers.go:56,70,72,74`, `keyvalue/tiers.go:55,64,68`), not resource names. **Aliases:** the GraphQL Service type is reachable as `server(id:)` **and** `service(id:)` — both bound to the same `s.Get` at `graphql.go:974-975` — plus the `services(ownerId:)` list at `:960` and the `SyncBlueprintResult.services` list at `:932`; they share `serviceGQLType`, so the one-line fix covers all four, but t002 must confirm rather than assume.
- **Consumer check (the fix must not break it):** the dashboard does **not** rely on GraphQL's `name`. `dashboard/src/features/services/lib/status.ts:35-39` re-implements `renderServiceName` client-side — `const immutableName = s.name ?? s.id ?? ""` then `name: displayName ?? immutableName` — and `service-detail-loader.ts:103` does the same. That is why the dashboard shows `eden-dash-v3` everywhere (heading, breadcrumb, page title, home list, notification override dropdown — all confirmed live this run) **despite** GraphQL, not because of it: a caller-level workaround that makes the backend look correct through a browser. `immutableName` has exactly two references, both inside `status.ts`, so making GraphQL's `name` the display label leaves the rendered result identical. `ServiceView.name`'s own doc comment already declares the settled semantics: "Human-facing label: displayName when set, otherwise the immutable App name."
- **Adjacent classes:** t004 changes a not-found boundary, so it must place the neighbours. Making `GetApp` resolve `displayName` adds a third resolution key that is **not unique** (nothing constrains two services in a workspace to distinct display names, unlike `LabelServiceName`), so an ambiguous match needs a defined answer — 409, first-match, or refusal — and it must not become an existence oracle for a caller who lacks access: a display-name miss must be indistinguishable from a display-name hit the caller cannot read. The alternative — leave addressing id-and-immutable-name only, matching Render — needs t003 to guarantee the immutable name is still readable, or the 404 becomes unrecoverable from an API response alone.
- **Unverified this run:** (1) the collided-slug case — every service in the QA workspace has `slug == name`, so `slug != name` was reasoned from `PlatformSubdomain` and the `service.go:346-353` doc comment, never observed; producing one needs a cross-tenant name collision and a second workspace. (2) MCP's `name` was read from the code path (`listServicesResult`/`renderServiceResult` → `renderService`), not called live — no MCP client was attached this run. (3) The webhook surface was not re-probed; its behavior is taken from `w6/m101`'s shipped fix and `c95a952d`'s test. (4) Whether any non-dashboard GraphQL consumer exists that would be affected.
