# CLI sweep 1253 — live REST responses against the pinned Render schema (live, 2026-10-20)

One free echo-server web service (env var, `.invalid` custom domain, one secret file) created with `bex services create`, then deleted (GET 404). Its responses, plus the read-only workspace lists and the baseline Postgres metadata, were validated with `jsonschema` against `render-public-api-1.json` (nullable-aware). No new bug filed.

- Clean: deploys, env-vars, custom-domains, instances, owners, projects, env-groups.
- Divergences, all already allowlisted in `lego/backend/internal/api/conformance_allowlist_test.go` with ADR018 citations: missing `rootDir`/`buildPlan`/`envSpecificDetails` on some service shapes (“partial serviceDetails projection”), `region: fsn1` (single-placement divergence), Postgres `project` omitted (“omitted unknown metadata”), secret-file list without `content` (“names without secret contents”), and the event-type/detail dialect. `GET /notification-settings/owners/{ownerId}` → 404 is ADR052's per-member design.
