# w6 · m130 — `renderSubdomainPolicy` is the third field in its family at the wrong REST nesting level

**Worker:** worker6 **Goal:** a Render-shaped client can set the platform-subdomain policy where Render's own schema declares it — inside `serviceDetails` — instead of getting `400 unknown field`, and no service type is told its non-existent subdomain is "enabled". **Status:** done — both defects fixed as the third instance of the disk (w1/m86) / ipAllowList (w6/m106) family: create + PATCH decode `serviceDetails.renderSubdomainPolicy` (top level still accepted, top level wins on create / nested wins on PATCH); it is emitted only inside `serviceDetails` and only for `web_service`/`static_site`, gated once in `view()` so REST omits it, MCP omits it, and GraphQL resolves it to `null` (`OptionalStrField`) for `private_service`/`background_worker`/`cron_job`; `setSubdomainPolicy` on a non-routable type is refused with a type-named 400 that does not mention custom domains. Decision recorded in `docs/ADR018-render-parity.md` row 117; covered by `lego/backend/internal/apps/subdomain_policy_test.go` (backend + api suites green, `make lint-backend` 0 issues).

## Tasks (in order)

| id   | title                                                                                  | est | depends_on | status   |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- | -------- |
| t001 | Accept `serviceDetails.renderSubdomainPolicy` on create + patch (top level still wins)   | 30m | —          | **DONE** |
| t002 | Decide + implement the REST response placement, and per-type reporting                   | 40m | t001       | **DONE** |
| t003 | Type-gate `SetSubdomainPolicy` so a non-routable service gets a reason it can act on     | 30m | t002       | **DONE** |
| t004 | Blast radius: every emission site enumerated, correct-today types regression-tested      | 40m | t002       | **DONE** |
| t005 | Render parity sweep (REST/GraphQL/MCP/dashboard)                                          | 30m | t003, t004 | **DONE** |
| t006 | Simplify                                                                                  | 20m | t005       | **DONE** |
| t007 | Test coverage                                                                             | 30m | t005       | **DONE** |
| t008 | Closeout                                                                                  | 10m | t007       | **DONE** |

## Background — two defects in one field, one file, one decision

### Defect 1 (request side): Render's documented nesting is rejected with 400

Render's pinned spec (`lego/backend/internal/api/openapi/render-public-api-1.json`) declares `renderSubdomainPolicy` inside `webServiceDetailsPOST`, `staticSiteDetailsPOST`, `webServiceDetailsPATCH` and `staticSiteDetailsPATCH` — nested in `serviceDetails`. It does **not** appear on the top-level `service` schema at all, whose complete property list is: `autoDeploy`, `branch`, `buildFilter`, `createdAt`, `dashboardUrl`, `environmentId`, `id`, `imagePath`, `name`, `notifyOnFail`, `ownerId`, `registryCredential`, `repo`, `rootDir`, `serviceDetails`, `slug`, `suspended`, `suspenders`, `type`, `updatedAt`.

bex accepts it **only** at the top level. Live probes, 2026-08-28, `fetch` from inside the authenticated dashboard page with `credentials:'include'` against `https://api.bex.co`. Each body is deliberately incomplete (`name:''`) so that unknown-field rejection — which happens during decoding — is distinguishable from later validation:

```
POST /v1/services {"type":"web_service","name":"","serviceDetails":{"env":"go","renderSubdomainPolicy":"disabled"}}
  -> 400 bad request: request body contains unknown field "renderSubdomainPolicy"

POST /v1/services {"type":"static_site","name":"","serviceDetails":{"publishPath":"dist","renderSubdomainPolicy":"disabled"}}
  -> 400 bad request: request body contains unknown field "renderSubdomainPolicy"

POST /v1/services {"type":"web_service","name":"","renderSubdomainPolicy":"disabled","serviceDetails":{"env":"go"}}
  -> 400 bad request: name must be a DNS label of 1-30 chars ([a-z0-9-])   <- DECODED FINE; top level is the accepted location

CONTROL — the field w6/m106 already fixed:
POST /v1/services {"type":"web_service","name":"","serviceDetails":{"env":"go","ipAllowList":[{"cidrBlock":"1.2.3.4/32","description":"x"}]}}
  -> 400 bad request: name must be a DNS label of 1-30 chars ([a-z0-9-])   <- DECODES at Render's nesting
```

The control is what makes this a defect rather than a design choice: `ipAllowList` — declared in exactly the same two Render detail schemas — decodes at the nested location **because `w6/m106` moved it**. Its sibling never was. Note the rejection is **not** type-gating: it fires for `web_service` and `static_site`, the two types Render declares the field on.

Root cause, both halves in one file:

- `lego/backend/internal/apps/rest.go:40` `patchServiceRequest` carries `RenderSubdomainPolicy *string` with tag `renderSubdomainPolicy` at `:95-98` — top level.
- `serviceDetailsReq` (same file, the nested struct shared by create and patch) has no such field. Its complete json-tag list is `plan`, `region`, `numInstances`, `healthCheckPath`, `maxShutdownDelaySeconds`, `runtime`, `env`, `envSpecificDetails`, `schedule`, `command`, `buildCommand`, `publishPath`, `preDeployCommand`, `previews`, `maintenanceMode`, `ipAllowList`, `disk`. Note `ipAllowList` (`w6/m106`) and `disk` (`w1/m86`) are present — the two prior fixes of this exact pattern — and `renderSubdomainPolicy` is not.

**The fix is the file's own established pattern, not a new invention.** Four adjacent fields in `serviceDetailsReq` are already dual-accept, documented in their own comments, e.g. "schedule is Render's cronJobDetails.schedule (accepted here or at the top level, top level wins)", with the same wording on `publishPath` and `preDeployCommand`.

### Defect 2 (response side): emitted top-level, and for every service type

Render emits it in `webServiceDetails`/`staticSiteDetails` only. bex emits it **both** at the top level of the service object and inside `serviceDetails`, for all five types. Live capture, 2026-08-28, `GET /v1/services/{id}`:

```
web_service        srv-da7o6ovvqdcc73bpn9hg  topLevel="enabled"  details="enabled"  details.url="https://qa-20260826-webhook-svc.onbex.co"
private_service    srv-da8e6u0ueu1c7395jo60  topLevel="enabled"  details="enabled"  details.url=<<absent>>
background_worker  srv-da8e73oueu1c7395joa0  topLevel="enabled"  details="enabled"  details.url=<<absent>>
cron_job           srv-da8e748ueu1c7395jobg  topLevel="enabled"  details="enabled"  details.url=<<absent>>
```

GraphQL agrees:

```
{ service(id:"srv-da8e73oueu1c7395joa0"){ type url renderSubdomainPolicy internalAddress } }
  -> {"type":"background_worker","url":"","renderSubdomainPolicy":"enabled","internalAddress":""}
```

So the same payload says "the platform subdomain is enabled" and "there is no url", for a type that provably has no route. Verified at the edge this run — all three non-routable hostnames 404 on both ports with no cert issued, while a public control 200s:

```
curl -sSk https://qa-20260828-private.onbex.co/  -> 404   (:80 -> 404)
curl -sSk https://qa-20260828-worker.onbex.co/   -> 404   (:80 -> 404)
curl -sSk https://qa-20260828-cron.onbex.co/     -> 404   (:80 -> 404)
curl -sS  https://agentmarketcap-1.onbex.co/     -> 200   (control)
```

Without `-k` all three fail TLS entirely ("unable to get local issuer certificate" — no managed cert); the public control verifies normally.

Root cause:

- **Top-level emission:** `lego/backend/internal/apps/render.go:158` declares `RenderSubdomainPolicy string` on the `renderService` struct, set at `:290`. Its own doc comment claims it "is Render's renderSubdomainPolicy field … Always present and non-empty" — the claim the pinned spec contradicts.
- **Untyped details emission:** `render.go:318` puts `{"renderSubdomainPolicy", a.RenderSubdomainPolicy}` in `renderServiceDetails`'s plain omit-when-empty field table, which applies no type condition. Two fields immediately below it **do** carry type conditions — `url` is gated on `svcType != renderPrivateService`, `buildCommand` on `svcType == appv1alpha1.TypeStaticSite` — so type-conditioning is an established capability of this function that this field never got.

**The repo already wrote down the rule this breaks.** `render.go:159-163`, immediately below the offending declaration:

```go
// NOTE: Render's inbound IP allowlist is NOT a top-level service field. Its
// schema places ipAllowList inside serviceDetails (webServiceDetails /
// staticSiteDetails only), so it is emitted from renderServiceDetails(), not
// here — see the ipAllowList block there (w6/m106).
```

`renderSubdomainPolicy` has identical web+static-only scope and sits directly above that note, untouched by it.

### Why the conformance guard cannot catch either half

`lego/backend/internal/api/conformance_allowlist_test.go` allowlists, for both `retrieve-service` and `list-services`, the blanket entry `serviceDetails: does not match the documented union`. That one entry swallows every serviceDetails-shaped divergence for those operations. Separately, the extra **top-level** property cannot fail validation at all: the pinned `service` schema leaves `additionalProperties` unset, so OpenAPI permits unknown properties. Neither half is reachable by the existing guard — which is why this survived two prior passes of the same fix pattern. Any task that narrows that allowlist entry must confirm the remaining real divergences still pass.

## Definition of done

- `POST /v1/services` and `PATCH /v1/services/{id}` accept `serviceDetails.renderSubdomainPolicy` for `web_service` and `static_site`, the two types Render's `webServiceDetailsPOST`/`staticSiteDetailsPOST`/`…PATCH` schemas declare it on. Re-running the exact probe `POST {"type":"web_service","name":"","serviceDetails":{"env":"go","renderSubdomainPolicy":"disabled"}}` no longer returns `400 … unknown field "renderSubdomainPolicy"` but proceeds to normal validation, exactly as the `ipAllowList` control does today. The top-level form keeps working, and when both are sent the documented precedence (top level wins) is asserted by a test.
- The REST response's placement of `renderSubdomainPolicy` is whatever `t002` decided, that decision is written into `docs/ADR018-render-parity.md` row 117, and `render.go:155-158`'s comment no longer asserts something the pinned spec contradicts. If the decision is to move it, `GET /v1/services/{id}` for a web service reports it inside `serviceDetails`, so a Render-parity client reading the documented location finds it — the same assertion `w6/m106` shipped for `ipAllowList`.
- For `private_service`, `background_worker` and `cron_job`, `renderSubdomainPolicy` reports the value `t002` named as correct, and REST and GraphQL agree on it. Today all three report `"enabled"` on both surfaces while the same payload omits `url` and the hostname 404s at the edge — re-verify with the four-type capture and the `curl -sSk` edge checks above.
- `setSubdomainPolicy` on a private service, a background worker and a cron job is refused with a type-specific reason that does not instruct the caller to add a custom domain — verified live per type, not inferred from one. Today on a private service it returns "cannot be disabled without at least one custom domain", and the custom-domain add then returns "a private_service has no ingress and cannot have custom domains".
- The two types that behave correctly today, `web_service` and `static_site`, still accept and report the field and still gate `disabled` behind having at least one custom host — covered by regression tests, since they are correct today *because of* current behaviour and a global change could silently alter them.

## Source + Goal linkage

- **Source:** the open inbox note `.pm/w6/063.md` (46th `/qa-find-bugs` run, 2026-08-27; moved to `.pm/w6/done/063.md` on promotion), extended by the 68th `/qa-find-bugs` run, 2026-08-28, journey 10 (cron job / background worker / private service). Workspace `tea-d98210cbbpdc73dcrkvg` (Pro). Fixtures `qa-20260828-private` (`srv-da8e6u0ueu1c7395jo60`), `qa-20260828-worker` (`srv-da8e73oueu1c7395joa0`), `qa-20260828-cron` (`srv-da8e748ueu1c7395jobg`) — all three created, probed and deleted in the same visit (`deleteService: true`, gone from the list). Every probe above is a complete request + response pasted so it can be re-run; no screenshot is cited, because the evidence here is the wire format.
- **Goal linkage:** `docs/ADR018-render-parity.md` row 117 (`renderSubdomainPolicy`) and the Render-compatible REST contract of `docs/ADR006-bex-api.md`. The field itself shipped in `w7/m31`.
- **Precedent — extend, do not re-litigate.** This is the **third** instance of one fix pattern: `w1/m86` moved `Disk` to Render's nesting; `w6/m106` moved `ipAllowList`, its goal line reading "never at the top level of the service object, where a Render-parity REST client checking the documented location finds nothing"; and `renderSubdomainPolicy` — declared in the same two Render detail schemas as `ipAllowList` — was missed by both. `render.go:159-163`'s own NOTE states the rule and cites m106. Neither prior milestone covered this field; `w7/m31` shipped it top-level before that rule was articulated. Do not re-litigate whether the field should exist.
- **Expected outcome:** a Render-shaped client can set the platform-subdomain policy where Render's spec says to, and no service type is told its non-existent subdomain is enabled.
- **Why now:** journey 10's promise is that a private service is not publicly reachable. The routing half is genuinely correct — verified 404 at the edge on both ports with no cert, against a 200 public control. What is wrong is what the API says *about* it, on the one field that describes public exposure. `063` has been open since 2026-08-27 with its reporting question unanswered, and this run turned the same field's request side into a reproducible 400.
- **Render parity:** the standing task is **included** — REST request and response shapes change, and GraphQL/MCP must be checked alongside.
- **Blast radius:** `renderService` (`render.go:439`) serves REST detail **and** the list loop (`:454`); `renderServiceDetails` (`:298`) is called once from `:262` but reached by both. GraphQL registers the field separately at `apps/graphql.go:438`; MCP accepts it as `update_service` input at `apps/mcp.go:116`, provided-map `:737`, applied `:811`. Dashboard consumers are `dashboard/src/features/services/types.ts:187` → `custom-domains-section.tsx:92,169` → `platform-subdomain-section.tsx:18,23,28`, plus `use-subdomain-policy.ts` — all reading the **GraphQL** flat field.
- **Adjacent classes:** a `web_service` and a `static_site` (field is meaningful, must keep working); a `private_service` / `background_worker` / `cron_job` (no subdomain exists — `t002` names the value); `renderSubdomainPolicy: "disabled"` on a routable service **with** custom hosts (must stay permitted); and a routable service with **no** custom host (must keep its existing 400 guard, which is correct today).

## Unverified — carried from the hunt, do not present as observed

- **The PATCH nesting rejection was not probed.** `PATCH` from a browser origin throws `TypeError: Failed to fetch` — the CORS method-list issue already filed as `w6/m121` — so PATCH is established here by **code inspection only**: `patchServiceRequest` (`rest.go:40`) declares the field at top level (`:95-98`) and shares the same `serviceDetailsReq` that lacks it. Confirm PATCH against a non-browser client before treating it as reproduced.
- The **MCP** surface was not exercised live; its behaviour is read from `apps/mcp.go:116/:737/:811` only.
- That a real Render-parity client (official CLI, Terraform provider) actually sends the nested form on create is **reasoned from the pinned spec, not observed**. It establishes what the documented location is, not that a specific client uses it.
- `063`'s own residue stands: `w6/m46`'s DoD still owes the MCP and `render.yaml`-blueprint create surfaces and the first-deploy `Canceled` probe; none was touched this run.
- `lastSuccessfulRunAt` never appeared on the cron fixture. **Not a defect and deliberately not filed:** the fixture's command `./app` starts a long-running HTTP server and never exits, so no run could complete. The schedule itself is correct and was verified — `nextRunAt` advanced `2026-08-28T01:30:00Z` → `01:40:00Z` across the boundary, from `schedule: "*/10 * * * *"` created at `01:25:37Z`. Whether bex bounds a non-terminating cron run was not investigated.
