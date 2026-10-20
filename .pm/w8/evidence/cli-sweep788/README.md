# CLI sweep 788 — `healthCheckPath` create/PATCH roundtrip (live, 2026-10-19/20)

Disposable free web service created via REST, patched, deleted (204, then GET 404). No new bug filed.

- Correct create shape (from `render_openapi_test.go`): top-level `"image":{"imagePath":…,"ownerId":""}` — earlier probes with `imagePath` inside `serviceDetails` or at top level were refused by name (strict decode working as designed; harness error, not a bug).
- Create with `serviceDetails.healthCheckPath=/healthz789` → 201, path echoed in `serviceDetails`.
- `PATCH {"healthCheckPath":"/healthz789b"}` → 200; subsequent GET shows `/healthz789b`. Field is writable through the dedicated path (`lego/backend/internal/apps/rest.go:47`, `service.go` `SetHealthCheckPath`).
- Two self-inflicted harness failures along the way (missing token file, Cloudflare 403 on default Python UA) were diagnosed and fixed in the harness, not filed.
