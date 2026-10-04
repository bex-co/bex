# m150 live acceptance — 2026-10-02 (UTC 2026-10-03T06:24–06:33Z)

**Deployed image:** `ghcr.io/bex-co/bex-operator@sha256:e396cf3585ac250a25f2d947e7b2b8f309def52e7bb15cb7826d92078a4f27d9` on `bex-static-server` and `bex-api` (GitOps pin `20fb64867` → `1263d12ae`, which contains `ccf6478f7` and `906bbe629` and the requested `18958459c`). **Fixture:** free static site `qa-20261003-static-m150` / `srv-db09v02tm2ss7389qn5g` in `tea-d98210cbbpdc73dcrkvg`, Public Git `https://github.com/bex-co/bex` `main`, root `examples/static-site`, publish `.`, no build, auto-deploy off. One deploy `dep-db09v02tm2ss7389qn60` (live 06:24:24Z), never rebuilt.

**Transport note:** the Playwright browser was in use by a parallel agent, so saves were sent as the dashboard's own `SetStaticHeaders` / `SetStaticRoutes` / `DeleteService` GraphQL mutations from curl with the signed-in session. Public probes used `curl` against `https://qa-20261003-static-m150.onbex.co`.

## Eight wildcard rows (saved 06:25:19Z; marker live 06:25:31Z)

Rows: `/*` X-QA-All, `/qa-route` X-QA-Path, `/index.html` X-QA-Index, `/*.yaml` X-QA-Pattern, `/render.yaml` X-QA-Exact, `/**/*.yaml` X-QA-Nested, `/**/*` X-QA-Nested-All, `/nested/*` X-QA-Prefix. GET and HEAD returned identical results:

| Request | Status | X-QA headers |
| --- | --- | --- |
| `/render.yaml` | 200 | all, exact, **pattern** (no nested markers) |
| `/missing-r11.yaml` | 404 | all, **pattern** |
| `/nested/missing-r11.yaml` | 404 | all, **nested, nested-all**, prefix (no root-only pattern) |
| `/nested/` | 200 | all, **nested-all**, prefix |
| `/` | 200 | all |

## Exact ⇄ wildcard toggle of the same rule

- `/*.yaml` → `/render.yaml` (06:29:33Z, marker `m150-v2` live 06:29:46Z): `/render.yaml` GET/HEAD carry the pattern header; `/missing-r11.yaml` no longer does.
- Back to `/*.yaml` (06:29:48Z, `m150-v3` live 06:29:58Z): `/render.yaml` GET/HEAD and `/missing-r11.yaml` carry it again.
- After the toggle, REST `GET /v1/services/{id}/headers`, GraphQL `service.headers` and MCP `list_static_headers` all returned the same eight rows with the exact `/*.yaml`, `/**/*.yaml`, `/**/*`, `/nested/*` strings. Deploy list still one `create` deploy.

## Route controls

`setStaticRoutes` with redirect `/render.yaml`→`/index.html`, redirect `/qa-old`→`/index.html`, rewrite `/*`→`/index.html` (06:30:16Z), then without the rewrite (06:30:46Z), then restored (06:31:16Z), probing ~25 s after each save, GET and HEAD:

- `/render.yaml`: 200, no Location, all/exact/pattern headers, in all three states (existing file wins).
- `/qa-old`: 301 `location: /index.html` in all three states.
- `/qa-route`: 200 with X-QA-All + X-QA-Path, no X-QA-Index, in all three states (without the rewrite this is the accepted implicit SPA fallback, ADR029).
- `/nested/deep/x.yaml`: rewritten 200 with nested, nested-all and prefix headers while the catch-all exists; 404 with the same headers while it is removed.

## Deletion

`deleteService` 06:32:11Z → `true`. At 06:33:04Z: REST service and `/headers` both 404 `not_found`; `GET /v1/services?ownerId=…&name=qa-20261003-static-m150` → `[]`; GraphQL `service` null + `not found`. Exact-identity cluster check: the pre-delete App CR, `bex-static-…` Service, Ingress, Certificate `…-tls`, pull Secret and clone Secret are all gone, and none remain in `bex-system`. The object-store prefix was not inspected (no S3 credentials used). The session was revoked at the end of the run.
