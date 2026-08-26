# w6 · m106 — Service `ipAllowList` sits at the wrong REST nesting level: root, not `serviceDetails`

**Worker:** worker6 **Goal:** `GET`/`POST`/`PATCH` `/v1/services` place a web service's or static site's inbound `ipAllowList` inside `serviceDetails` exactly where Render's own pinned OpenAPI schema (`webServiceDetails`/`staticSiteDetails`) declares it, matching the fix pattern already shipped for `Disk` (`w1/m86`) in the same file — never at the top level of the service object, where a Render-parity REST client checking the documented location finds nothing. **Status:** todo

## Background (found live, 2026-08-26, 16th `/qa-find-bugs` hunt)

While testing the dashboard's per-service "Networking" section (`Restrict inbound HTTP traffic to these source CIDRs`), confirmed the feature **works end to end**: a saved CIDR persists, and traffic from an out-of-list IP genuinely gets `HTTP/2 403 Forbidden` (real Traefik `Middleware` enforcement, not a decorative saved value). But comparing REST against GraphQL for the same live resource surfaced a wire-shape bug:

**Live probe 1 — dashboard-saved CIDR, REST vs GraphQL on the same service (`srv-da7k16qpjh2s73adltbg`, deleted during cleanup):**

```
GraphQL: query { server(id:"srv-…") { ipAllowListEntries { cidrBlock description } } }
=> { "data": { "server": { "ipAllowListEntries": [{ "cidrBlock": "198.51.100.0/24", "description": "qa-check" }] } } }

REST:    GET /v1/services/srv-…   (full body, not just serviceDetails)
=> { …, "serviceDetails": { …no ipAllowList key at all… }, …, "ipAllowList": [{ "cidrBlock": "198.51.100.0/24", "description": "qa-check" }] }
```

REST **has** the value — but at the JSON root, as a sibling of `serviceDetails`, not inside it.

**Live probe 2 — REST create with Render's own documented nested input, to rule out a write-path issue (`srv-da7k23fkh5kc73fbqt90`, deleted during cleanup):**

```
POST /v1/services  { "type":"web_service", …, "serviceDetails": { "plan":"free", "ipAllowList":[{"cidrBlock":"192.0.2.0/24","description":"rest-create-test"}] } }
=> 201 { "service": { …, "serviceDetails": { …no ipAllowList key… }, …, "ipAllowList": [{"cidrBlock":"192.0.2.0/24","description":"rest-create-test"}] }, "deployId": "…" }
```

REST **accepts** the field nested (matching Render's real create contract) but **always emits** it at the root on both create and get — an asymmetric wire shape, live-reproduced on both the create response and a subsequent GET.

**Ground truth, not guessed.** This repo embeds Render's actual pinned public OpenAPI spec (`lego/backend/internal/api/openapi/render-public-api-1.json`, fetched 2026-07-20, sha256-pinned in `lego/backend/internal/api/render_openapi.go:39`). Checked directly:

```
webServiceDetailsPOST.properties.ipAllowList  -> present (array of cidrBlockAndDescription)
webServiceDetailsPATCH.properties.ipAllowList -> present
webServiceDetails.properties.ipAllowList      -> present   (the GET/response shape)
staticSiteDetails.properties.ipAllowList      -> present
privateServiceDetails / backgroundWorkerDetails / cronJobDetails -> ipAllowList absent from all three
```

Render's own contract places `ipAllowList` inside `serviceDetails`, scoped to exactly `web_service`/`static_site` — matching `w7/m32`'s own milestone title ("Service inbound ipAllowList: web services + static sites") and the existing create-time gate that already rejects the field on a non-ingress type (`lego/backend/internal/apps/deploy.go:2025-2026`).

**Root cause.** `lego/backend/internal/apps/render.go:150-157` declares `IPAllowList []ipAllowEntry \`json:"ipAllowList,omitempty"\`` as a field on the top-level `renderService` struct (a sibling of `ServiceDetails map[string]any`, `render.go:81`), populated at `toRenderServiceWithMetadata` (`render.go:220-286`, specifically line 286: `IPAllowList: cloneIPAllowEntries(a.IPAllowList)`). `renderServiceDetails()` (`render.go:293-360`), which builds the actual `serviceDetails` map, never adds an `"ipAllowList"` key — so every REST response (create, PATCH, get, list — all route through the same `toRenderServiceWithMetadata`/`renderServiceDetails` pair) places the value at the wrong nesting level.

**The exact fix pattern already exists in the same file, for the same bug class.** `render.go:346-353` (comment: "Render's schema puts the attached disk inside serviceDetails … not beside it — so a Render client reads serviceDetails.disk and finds nothing if it is only on the sibling view (ADR082 D6; the omission was caught by the w1/m86 parity audit)") shows this exact mistake was already made and fixed once, for `Disk` — and `Disk` has **no** top-level sibling field on `renderService` at all; it lives only inside `serviceDetails`. `IPAllowList` needs the identical treatment: move (not duplicate) into `renderServiceDetails()`, gated to `svcType == web_service || svcType == static_site` (mirroring how `maintenanceMode` is already gated to `renderWebService` only, a few lines above the Disk block).

**Not a write-path or store-desync bug.** Traced independently (two research passes, one that initially concluded "no bug — REST does return it, just at the root" before the second pass found the write/read asymmetry against Render's actual schema): both the dashboard's GraphQL mutation (`setServiceIpAllowList` → `lego/backend/internal/apps/service.go:3550-3557`) and REST's own POST/PATCH decode path (`rest.go:501-504`, `:792-797`) write to the identical live App CR field (`AppSpec.IPAllowListEntries`, `lego/types/v1alpha1/app_types.go:672-686`), which the operator projects into a real Traefik `Middleware` (`lego/operator/internal/controller/app_controller.go:2310-2339`) — enforcement and every other surface are correct. Only REST's *response-rendering* code (`render.go`) puts the value at the wrong JSON nesting level.

**GraphQL and MCP are unaffected — confirmed by grep, not assumed.** Every other reader of `AppView.IPAllowList` goes through a completely separate code path that never touches `render.go`: GraphQL's `ipAllowList`/`ipAllowListEntries` fields resolve straight off the view struct (`lego/backend/internal/apps/graphql.go:485,489`), and MCP's `get_service`/`update_service` do the same (`lego/backend/internal/apps/mcp.go:249,481`). `render.go:286` is the **only** place that constructs the wrong shape, and `ip_allow_list_test.go`'s `TestRESTCreateWithIPAllowListAndReadBack` (lines 154-163) is the **only** existing test covering this response shape — and it passes today because it asserts the top-level field, encoding the bug rather than catching it.

**Why `docs/ADR018-render-parity.md` didn't catch this.** Line 116 states: "REST: create/PATCH/GET use Render's exact ordered `[{cidrBlock,description}]` shape, proven through unmodified CLI v2.21.0" — this claim is inaccurate as written. It is not deploy-lag (`git log -S"IPAllowList" -- lego/backend/internal/apps/render.go` shows no fix in flight on `main`) and not a re-read error on this run's part after the second, careful trace (the first trace pass made exactly the mistake the ledger line encourages — trusting the "documented, tested" framing over an independent read of Render's actual schema).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Move `ipAllowList` from `renderService`'s top-level field into `renderServiceDetails()`'s map, gated to `web_service`/`static_site` exactly like `disk` and `maintenanceMode`; decide and record whether the legacy root-level field is dropped outright or kept one release as a deprecated alias | 30m | — |
| t002 | Regression tests: extend `TestRESTCreateWithIPAllowListAndReadBack` to assert the nested `serviceDetails.ipAllowList` location (create, PATCH, get, list); add a negative test that a `private_service`/`background_worker`/`cron_job` never carries `ipAllowList` in `serviceDetails` even if a caller tries to set it | 40m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 20m | t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- `GET /v1/services/{id}` for a web service or static site with a saved `ipAllowList` shows it at `serviceDetails.ipAllowList` as `[{cidrBlock, description}]`, matching Render's pinned schema (`webServiceDetails`/`staticSiteDetails` in `lego/backend/internal/api/openapi/render-public-api-1.json`) — live-verifiable: an authenticated `GET`/browser fetch shows the field nested, not at the JSON root (or, if t001 keeps a transitional root-level alias, both agree byte-for-byte and the root copy is documented as deprecated, not silently duplicated forever).
- `POST /v1/services` (create) and `PATCH /v1/services/{id}` responses show the identical nested shape as a subsequent `GET` — no create-vs-get divergence (live-verifiable with the exact request/response pair captured in this hunt).
- A `private_service`, `background_worker`, or `cron_job` never carries `ipAllowList` in `serviceDetails`, matching Render's schema (`privateServiceDetails`/`backgroundWorkerDetails`/`cronJobDetails` have no such property) and the existing create-time rejection (`lego/backend/internal/apps/deploy.go:2025-2026`).
- GraphQL (`ipAllowList`/`ipAllowListEntries`) and MCP (`get_service`/`update_service`) continue to agree with REST's corrected values — confirmed unaffected by this fix (only REST's nesting changes), not re-broken by it.
- `TestRESTCreateWithIPAllowListAndReadBack` and the new negative test pass; `cd lego/backend && go test ./...` green.
- `docs/ADR018-render-parity.md:116` is corrected to describe the actual (fixed) nested shape, without silently dropping the useful "web + static, `{cidrBlock,description}`, Traefik-enforced" content already there.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 16th run, 2026-08-26. Evidence: live REST `GET`/`POST` responses and GraphQL query pasted above, against fresh `qa-`-prefixed fixtures (`srv-da7ju4vkh5kc73fbqsp0`, `srv-da7k16qpjh2s73adltbg`, `srv-da7k23fkh5kc73fbqt90` — all created, tested, and deleted during this hunt) plus Render's own pinned OpenAPI spec checked directly in-repo (`lego/backend/internal/api/openapi/render-public-api-1.json`).
- **Goal linkage:** [ADR006](../../../docs/ADR006-bex-api.md) (bex-api Render-compatibility) and [ADR018](../../../docs/ADR018-render-parity.md) (parity ledger) — a REST client that trusts bex's own documented "exact Render shape" claim and reads `serviceDetails.ipAllowList` (as Render's real API requires) currently gets no signal that a service's inbound access is restricted, even though it demonstrably is. Enforcement itself is correct and unaffected; this is a wire-contract-honesty fix.
- **Expected outcome:** a REST client following Render's real, documented contract sees the same inbound-restriction state the dashboard and GraphQL already show correctly, with no change to enforcement, GraphQL, MCP, or the dashboard UI.
- **Why now:** cheap, precisely diagnosed to file:line, with a proven fix pattern already shipped once in the same function for the identical bug class (`Disk`, `w1/m86`) — low risk, and the current state contradicts a claim the project's own parity ledger asserts as verified.
- **Render parity task included:** yes (t003) — this milestone exists specifically to close a Render-parity gap; t003 also re-confirms GraphQL/MCP remain correct post-fix and updates the ledger claim.
