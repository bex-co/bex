# w4 · m121 — A service's port is create-only and unreachable from the dashboard, so bex's own `PORT` refusal names a control that does not exist

**Worker:** worker4 **Goal:** the sentence bex returns on every surface — "bex sets it from the service port; change the service port instead" — names a control the caller can actually reach: the port is settable in the dashboard create wizard, editable afterwards on every write surface, and readable back on REST, GraphQL and MCP. **Status:** todo

## Tasks (in order)

| id   | title                                                                                | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | Add the missing update verb: a service's port becomes editable on REST/GraphQL/MCP    | 50m | —                |
| t002 | Make the port readable: `serviceDetails.port`, GraphQL `Service.port`, MCP service read | 30m | w4/m121/t001     |
| t003 | Dashboard: a port control in the create wizard and in Settings, linked from the `PORT` refusal | 50m | w4/m121/t002     |
| t004 | Decide and record the Blueprint half, and make the refusal sentence name a reachable control on every surface | 30m | w4/m121/t003 |
| t005 | Blast radius: prove a port change re-reconciles Service/Ingress/probe, and keep the portless types portless | 40m | w4/m121/t004 |
| t006 | Render parity — REST/GraphQL/MCP/UI agree on the port field, and ADR018 row 117 is corrected | 30m | w4/m121/t005 |
| t007 | Simplify — `/simplify` over the code this milestone changed                            | 25m | w4/m121/t006     |
| t008 | Test coverage — the port write/read/refusal behaviors this milestone shipped           | 40m | w4/m121/t006     |
| t009 | Closeout — close the milestone once the definition of done actually holds              | 15m | w4/m121/t008     |

## Definition of done

Each bullet is a click or a command the next person can repeat.

- **The dashboard can host an image that listens on a port other than 3000, with no call made outside the browser.** In `/services/new?type=web_service` → **Existing Image**, `docker.io/traefik/whoami:latest` with `WHOAMI_PORT_NUMBER=8080` and the new port control set to `8080` reaches **Live**, and `curl -sSI https://<name>.onbex.co/` from a terminal answers `200`. Today the same wizard offers no port control at all, and the equivalent service created through it on `2026-09-21` answered `503`.
- **The port is editable after create on REST.** `PATCH /v1/services/<id>` carrying the port field returns `200`, not today's `400 {"error":"bad request: request body contains unknown field \"port\""}`, and a following `GET /v1/services/<id>` reports the new value.
- **The port is readable on all three API surfaces.** `GET /v1/services/<id>` returns `serviceDetails.port`; GraphQL `{ server(id:"srv-…"){ port } }` returns it instead of today's `Cannot query field "port" on type "Service".`; the MCP service read returns it. All three report the same number, and it agrees with `serviceDetails.internalAddress`'s `<slug>:<port>` suffix — which is, today, the only place a caller can read the port at all.
- **A port change actually moves the running service.** On a service created at `8080`, changing it to `8081` (container following) opens a deploy; afterwards the k8s Service targetPort, the Ingress backend port, the health probe port and `internalAddress` all read `8081`, and the platform URL still answers `200`.
- **The refusal now names something reachable.** Typing `PORT` into either dashboard env editor (service Environment tab and the env-group editor) still shows `core.ReservedEnvKeySentence` and still blocks Save — and the sentence's "change the service port instead" resolves to a control on a page the user can open from there; clicking through lands on it. On REST/GraphQL/MCP the coded `ENVIRONMENT_VARIABLE_RESERVED` body names the field and verb that changes it.
- **The portless types stay portless.** `background_worker`, `cron_job` and `static_site` render no port control on any dashboard surface, and a `port` on their create/update is refused or ignored exactly as today — asserted by a per-type table test covering all six resource types that reach `specFromCreate` (web · private · worker · cron · static, plus the image-runtime variant of web).
- **The ledger tells the truth.** `docs/ADR018-render-parity.md` row 117 no longer rests on an alternative that does not exist: it records where the port is set and changed, on which surfaces, and what Blueprint does (t004's decision). `docs/ADR004-app-deployment.md`'s `envVars` note points at the same place.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 106 (w4-targeted, `muse.env` credentials), browser-driven against `https://dashboard.bex.co`. Direct follow-up to **`w2/m95`**, whose own goal line promised "a migrating Render user learns immediately, with a code, that bex owns `PORT` **and where to change the service port instead**" (`.pm/w2/blocked/m95/README.md:120`) — the code shipped, the "where" never did. Not a duplicate of m95: m95 is blocked on live verification of the refusal it shipped; this milestone builds the control that refusal points at.
- **Goal linkage:** ADR008 pillar 1 (a Render-compatible hosting product) and `docs/ADR018-render-parity.md` row 117, which records refusing `PORT` as a **deliberate** divergence from Render — a divergence that is only defensible if bex's named alternative is reachable. ADR004 (`envVars`) owns the injection invariant this preserves.
- **Expected outcome:** an off-the-shelf container image that binds a fixed port — the majority of public images, including the wizard's own placeholder `docker.io/library/nginx:latest` — becomes hostable end to end from the dashboard, and stops being a dead end for a user who reads bex's own error message and looks for the setting it names.
- **Why now:** the gap makes a first-run journey fail silently. `Existing Image` is offered as a first-class source on the create wizard and is the one source with no build step to reinterpret the port, so the wizard's promise ("Deploy a web service from a Git repo or Docker image") is false for any image not already listening on 3000. The workaround exists but is invisible: `port` is accepted at create by REST (`rest.go:191`), GraphQL (`graphql.go:1326`) and MCP (`mcp.go:246`) and by nothing the dashboard sends — so the product's most capable path is the one no dashboard user can find.
- **Render parity task included** because this changes REST, GraphQL, MCP and the dashboard together. Render has no port _setting_ — it detects the bound port and lets the user override `PORT` as an ordinary variable — so parity here is about the divergence being honestly recorded and the alternative being real, not about copying a Render field. t004 owns that judgement.

## Evidence (live, 2026-09-21)

Probes, so the next person can re-run them rather than trust a screenshot (`.playwright-mcp/` is gitignored).

**1. The dashboard create wizard sends no port.** Captured `CreateService` GraphQL variables from the wizard submission:

```json
{
  "image": "docker.io/library/nginx:alpine",
  "name": "qa-20260921-img",
  "type": "web_service",
  "registryCredentialId": "",
  "runtime": "image",
  "plan": "free",
  "ownerId": "tea-d98210cbbpdc73dcrkvg"
}
```

Result: `nginx:alpine` (binds `:80`) got `PORT=3000`; `curl -sS -o /dev/null -w "%{http_code}" https://qa-20260921-img.onbex.co/` → **`503`**.

**2. The API can do what the dashboard cannot.** Same workspace, same minute:

```graphql
mutation {
  createService(
    name: "qa-20260921-port"
    ownerId: "tea-d98210cbbpdc73dcrkvg"
    type: "web_service"
    image: "docker.io/traefik/whoami:latest"
    runtime: "image"
    plan: "free"
    port: 8080
    envVars: [{ key: "WHOAMI_PORT_NUMBER", value: "8080" }]
  ) {
    id
    name
    url
  }
}
```

→ `{"data":{"createService":{"id":"srv-daodndjs0ils73bgpcng","name":"qa-20260921-port","url":"https://qa-20260921-port.onbex.co"}}}`, and `curl -sS -o /dev/null -w "%{http_code}" https://qa-20260921-port.onbex.co/` → **`200`**.

**3. The port is unreadable.** Adding `port` to that same selection set:

```
{"data":null,"errors":[{"message":"Cannot query field \"port\" on type \"Service\"."}]}
```

`GET /v1/services/srv-daodndjs0ils73bgpcng` → `200`, and its `serviceDetails` carries `"internalAddress":"qa-20260921-port:8080"` but **no** `port` key.

**4. The port is unchangeable.** On a third throwaway service created at `8080`:

```
PATCH /v1/services/srv-daodorp2dbts73fi2jvg   body: {"serviceDetails":{"port":9090},"port":9090}
→ 400 {"error":"bad request: request body contains unknown field \"port\"", "id":"bad_request"}
GET  /v1/services/srv-daodorp2dbts73fi2jvg → serviceDetails.internalAddress = "qa-20260921-patch:8080"  (unchanged)
```

All three `qa-20260921-*` services were deleted (`204`) and are absent from the dashboard overview.

## Root cause

- `lego/backend/internal/core/config.go:105-107` — `ReservedEnvKeySentence` returns `environment variable %q is reserved: bex sets it from the service port; change the service port instead`. Every surface renders this exact string (REST, GraphQL, MCP, Blueprint validation, both dashboard editors), so the dangling instruction is the product's, not one surface's copy bug.
- `lego/backend/internal/apps/service.go` — **23** `func (s *Service) Set…` mutators, and `grep -rn --include='*.go' 'func .*SetPort' lego/` returns **0**. There is no update path for the port on any surface. `applyCreateToSpec` (`service.go:2772`) copies `dst.Port = want.Port`, but only create reaches it; `normalizeCreateDefaults` (`service.go:2513-2522`) is the sole place a port is chosen, defaulting to `appv1alpha1.DefaultPort`.
- `lego/backend/internal/apps/rest.go:191,548` (create only), `graphql.go:1326` (`createService(port:)`), `mcp.go:246` (`create_web_service` args) — port is create-only on all three. `patchServiceRequest` (`rest.go:321` and the struct above it) has no port field, which is why PATCH answers `unknown field "port"`.
- `dashboard/src/features/services/` — the wizard's image tab renders `services.createImagePortHint` ("The container must listen on `$PORT` (default 3000)…", `locales/en.ts:2798`, shown via `service-source-picker.tsx:282`) and the env editors render `services.…` / `env-groups.…` reserved-key text (`services/locales/en.ts:740`, `env-groups/locales/en.ts:43`, gate in `lib/environment-draft.ts:5-7`, `components/create-env-var-editor.tsx:56`) — but no file under `dashboard/src` reads or writes a service port. The full-page `innerText` of `/services/<id>/settings` contains exactly one occurrence of "port" (inside the health-check helper text), across all eight settings sections.
- `lego/operator/internal/controller/app_controller.go:3994-4026` — `appEnv` strips a user `PORT` from `spec.env` and appends the operator's own from `spec.Port`. That invariant is correct and stays; it is the reason the create-only field is the _only_ lever.

## Unverified this run

- `private_service` port behavior was reasoned about from shared code, never probed live.
- Blueprint (`render.yaml`) has no `port` key today (`grep -w port lego/backend/internal/apps/blueprint.go` → 0 hits); no manifest was applied to confirm what a manifest author sees.
- MCP `create_web_service` port acceptance was read in code (`mcp.go:246`), not exercised against the live MCP endpoint.
- The `8080 → 8081` re-reconcile in the DoD has never been run — it is t005's work, not an observation.
