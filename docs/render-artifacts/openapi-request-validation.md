# Render OpenAPI request validation

This artifact defines the REST request boundary introduced in w7/m49. It is the review checklist for changing either bex routes or the pinned Render contract; it is not a claim that render.com rejects every input bex rejects.

## Pinned contract

| property | value |
| --- | --- |
| upstream | `https://api-docs.render.com/openapi/render-public-api-1.json` |
| fetched | 2026-07-20 |
| checked-in asset | `lego/backend/internal/api/openapi/render-public-api-1.json` |
| OpenAPI / info version | 3.0.2 / 1.0.0 |
| size | 331,391 bytes |
| SHA-256 | `2d27d5834d8bbc586e0aee62160cf996bb07f4be747112f15ec02c14fc11b315` |
| coverage | 130 paths · 207 operations · 58 request bodies · 163 component schemas |
| validator | `github.com/getkin/kin-openapi` v0.142.0 |

The API embeds this file and verifies its digest while constructing `Server.Handler()`. Loading and document validation happen once, offline. External references are disabled. The only upstream document compatibility option is `AllowExtraSiblingFields("description", "default")`; a guard test pins the three exact `$ref` objects that need it. The in-memory server list is cleared so validation accepts bex hosts, and a cloned request loses exactly one leading `/v1` before kin routing. The request delivered to the handler is not rewritten.

There is one contract asset and one loader. The response conformance suite uses the same parsed document through kin; response validation remains test-only and never buffers production responses or SSE.

## Request path and ownership

The production order is:

`body limit → OAuth/API-key authentication → caller rate limit → actual bex mux match → Render OpenAPI match → declared-constraint validation → strict Go decode → existing Core/domain validation`

Only the intersection of an actual authenticated bex mux match and a pinned Render operation is enforced. `http.ServeMux.Handler` proves bex owns the method/path first; kin then resolves the cloned `/v1`-adjusted request. This preserves two important boundaries:

- A Render operation bex does not implement keeps bex's existing 404/405 instead of becoming an OpenAPI 400.
- A bex route or alias absent from Render passes through unchanged and does not receive strict JSON context.

The operation-ID inventory guard currently finds exactly 119 enforced operations. `TestRenderRouteIntersectionInventory` pins the complete sorted list, so a route or upstream-spec refresh cannot silently enter or leave the boundary.

### Route-family classification

| Render family | classification | current boundary |
| --- | --- | --- |
| Services, service instances, autoscaling, one-off jobs, cron current-run verbs, static routes/headers, notification overrides | enforce where implemented | CRUD, lifecycle, scale, instances, jobs, current cron run, route/header, and per-service notification operations in the 119-operation guard |
| Deploys | enforce where implemented | list/get/create/cancel/rollback |
| Environment variables, secret files, environment groups | enforce where implemented | service and group CRUD/link operations |
| Custom Domains | enforce where implemented | create/list/get/delete/refresh |
| Events, Logs, Metrics | enforce where implemented | service events; log query/value/SSE subscription; implemented metrics selected by the `/v1/metrics/` handler |
| Postgres | enforce where implemented | CRUD, suspend/resume/restart/failover, connection info, and exports |
| Key Value | enforce where implemented | canonical Key Value CRUD, suspend/resume, and connection info |
| Projects & Environments | enforce where implemented | CRUD and official service/environment-group association operations |
| Registry Credentials | enforce canonical spelling | `/v1/registrycredentials`; the retired hyphenated spelling returns 404 |
| Webhooks | enforce where implemented | endpoint CRUD plus event reads; the inbound git webhook is public/HMAC and outside the gate |
| Blueprints | enforce where implemented | list/get/validate; bex-only sync spelling remains pass-through |
| Users and Workspaces | enforce where paths overlap | user, owner/workspace list/get/member reads; bex member/invite management routes remain pass-through |
| Workflows | enforce list stub only | the official CLI probe receives `[]`; workflow details/tasks remain unsupported |
| Audit Logs, Dedicated IPs, Disks, Maintenance, general Notification Settings, unsupported Postgres/Key Value/Blueprint/Workflow operations, deprecated Redis operations | Render operation not implemented | existing 404/405 or existing bex route behavior; no spec-only validation |
| Retired `/v1/apps`, `/v1/databases`, `/v1/registry-credentials`, `/v1/webhooks/endpoints`, Postgres `/exports`, and GET recovery-info | retired public alias | rejected with 404 before dispatch; the private port-8091 control-plane `/v1/apps` API is separate and unchanged |
| Datastore-specific logs/IP-allowlist/users/query routes, API keys, usage, tenant/workspace management, GitHub connect, shell tickets, cron history, deploy-hook management, and other bex extensions | bex extension pass-through | handler behavior remains byte-compatible; strictness is not inferred from a similar Render noun |
| CLI device grant/token/refresh, inbound HMAC git webhook, tokenized deploy hooks | public credential-specific route | mounted outside OAuth and outside this validator |
| GraphQL, MCP, health/discovery, SSE response stream | non-REST or response boundary | GraphQL/MCP are not checked with the REST contract; the SSE request is checked but its stream is never response-validated |

The table is family-level documentation; the exact method/path/operation-ID truth is executable in `render_openapi_test.go`.

## Compatibility policy

### Requiredness preserved from existing bex behavior

All fields remain type- and constraint-checked when supplied. Only these official required fields are made optional in the in-memory request document:

| operation ID | optional in bex | reason |
| --- | --- | --- |
| `create-service` | `ownerId`, `type` | selected/default workspace and `web_service` defaults |
| `create-postgres` | `ownerId`, `plan`, `version` | selected/default workspace and platform defaults |
| `create-key-value` | `ownerId`, `plan` | selected/default workspace and platform default plan |
| `create-env-group` | `ownerId`, `envVars` | selected/default workspace and empty-group creation |
| `create-project` | `environments` | empty project creation |
| `create-webhook` | `ownerId`, `enabled`, `eventFilter` | selected/default workspace and existing webhook defaults |
| `list-logs`, `list-logs-values`, `subscribe-logs` | query `ownerId` | selected/default workspace |

Render's overlapping `serviceDetails` request `oneOf` branches are evaluated as `anyOf` for service create/update; every branch constraint remains active. Bex's hyphenated Postgres plan IDs are added to only the create/update plan enums. A missing `Content-Type` on a request with a body is treated as JSON for compatibility with existing clients; an explicitly wrong media type is rejected.

### Bex request extensions on enforced operations

Render leaves its object schemas open, so the existing Go request structs remain the authoritative accepted body shape. Strict decoding therefore accepts these reviewed extensions while rejecting everything else:

| operation family | accepted extension |
| --- | --- |
| service create | `builder`, `port`, top-level `plan`, `domains`, top-level `publishPath`, top-level `preDeployCommand`, `routes`, `headers`, body `dryRun` |
| service update | `displayName`, top-level notification/subdomain/cron/health/pre-deploy conveniences, body `dryRun`, `serviceDetails.idleTTLSeconds` |
| Postgres create | `public`, `pooler`, body `dryRun`; bex hyphenated plan IDs |
| Postgres update | `version`, body `dryRun`; bex hyphenated plan IDs |
| Key Value create | `version`, `storageGB`, `public`, body `dryRun` |
| Key Value update | body `dryRun` |
| service, Postgres, Key Value create/update query | `dryRun` |
| protected service/Postgres/Key Value delete and lifecycle query | `confirm` on the operation IDs listed in `renderQueryExtensions` |

Declared Render fields that bex cannot honor are rejected by the existing Go/domain layer rather than silently accepted. Adding an extension requires all three: an accepting Go request field, this table, and a positive boundary test. Adding a query extension additionally requires an operation-ID entry in `renderQueryExtensions`.

## Rejection and secrecy rules

Kin validates declared path/query/body types, required fields, enums, formats, patterns, ranges, and media types with format validation enabled, default injection disabled, and fail-fast errors. The wrapper separately rejects:

- a body on an operation with no request body;
- query names absent from the path/operation parameters and reviewed query-extension map;
- top-level or nested JSON fields absent from the existing typed Go request;
- a second JSON value or trailing JSON token.

The validator never returns or logs kin's raw error string. Those errors can contain submitted values and a full schema dump. A rejection uses the shared Render-shaped `{id,error,message}` 400 envelope and names at most the parameter or JSON pointer. Strict decode errors may name a bounded safe field name, never its value. Validation does not trim, normalize, default, or otherwise rewrite input, and the handler receives the exact bounded body bytes kin read.

Unknown-field rejection is a deliberate bex security extension. Render's public schemas mostly omit `additionalProperties:false`, and this document does not claim render.com's live API makes the same rejection.

## Manual pin refresh

Render describes the source as unversioned, so refresh is deliberate and review-gated. Never fetch it at API startup or in ordinary CI.

1. Fetch to a disposable directory and record provenance:

   ```sh
   render_spec_tmp="$(mktemp -d)"
   curl -fsS https://api-docs.render.com/openapi/render-public-api-1.json \
     -o "$render_spec_tmp/render-public-api-1.json"
   wc -c "$render_spec_tmp/render-public-api-1.json"
   shasum -a 256 "$render_spec_tmp/render-public-api-1.json"
   jq -r '[.openapi, .info.version, (.paths|length), (.components.schemas|length)] | @tsv' \
     "$render_spec_tmp/render-public-api-1.json"
   ```

2. Review the full diff against `lego/backend/internal/api/openapi/render-public-api-1.json`. Recount operations/request bodies, external `$ref` values, and non-extension `$ref` siblings. Do not broaden loader exceptions to make an unexplained change pass.
3. Review the 119-operation intersection, requiredness compatibility map, query-extension map, accepted body table, newly implemented/unsupported operations, and official CLI request shapes.
4. Deliberately replace the asset, then update the source date, byte count, SHA constant, exact guard counts/sibling list, and this artifact in the same change.
5. From `lego/backend`, run:

   ```sh
   go test ./internal/api -run 'TestRender(OpenAPI|Route|Request|Conformance)|TestConformanceAllowlistEntries' -count=1
   go test ./...
   ```

6. Run the relevant lint target and the official CLI compatibility fixtures. Any new response divergence needs a narrow ADR018-cited allowlist entry; the stale-entry test must remain green. Do not add runtime response validation.

The digest/count/ref/intersection tests intentionally fail after replacement until the review metadata is updated. That failure is the refresh gate, not a reason to loosen the tests.
