# w5 · m582 — Exhaustive declared-request-field and composed-body-gate coverage across all ~58 served JSON-body operations

**Worker:** worker5 **Goal:** wire Render's `environmentId` on `POST /v1/postgres/{id}/recovery` (`recover-postgres`, plus GraphQL/MCP) and expand `TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName` (currently 6 ops, `webServiceDetailsPATCH` only) and `TestRenderGatedBexBodiesReachTheirHandlers` (currently 28 ops) to cover every served Render JSON-body operation and all five `serviceDetailsPATCH` variants, fixing every `unknown field` or gate/handler mismatch uncovered. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Wire `environmentId` on `POST /v1/postgres/{id}/recovery` (`recover-postgres`, plus GraphQL/MCP `Recover`) instead of refusing it as unsupported | 1h | — |
| [t002](t002.md) | Expand `TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName` to probe every served Render JSON-body operation and all five `serviceDetailsPATCH` variants from the pinned OpenAPI spec | 1h30m | t001 |
| [t003](t003.md) | Fix every `unknown field` refusal or gate/handler mismatch surfaced across the newly probed body operations | 1h | t002 |
| [t004](t004.md) | Expand `TestRenderGatedBexBodiesReachTheirHandlers` from 28 to every served Render JSON-body operation with an OpenAPI inventory guard and update `openapi-request-validation.md` | 45m | t003 |
| [t005](t005.md) | Render parity | 20m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 15m | t007 |

Total: **8 tasks, about 5h35m**.

## Definition of done

1. `POST /v1/postgres/{id}/recovery` (and GraphQL/MCP recovery) accepts Render's `environmentId`, validates workspace membership and protected-environment Admin authorization identically to `POST /v1/postgres`, places the recovered database in that environment, and preserves source-environment inheritance when `environmentId` is omitted.
2. `TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName` (`lego/backend/internal/api/render_declared_fields_test.go`) derives its probe set from the pinned OpenAPI document so every served JSON-body operation and all five `serviceDetailsPATCH` variants (`web`, `private`, `worker`, `static`, `cron`) are probed with `unprobed == 0` and zero generic `unknown field` responses.
3. `TestRenderGatedBexBodiesReachTheirHandlers` (`lego/backend/internal/api/render_gated_bodies_test.go`) covers every served Render JSON-body operation and fails if a newly mounted body operation is omitted from its table.
4. `docs/render-artifacts/openapi-request-validation.md` and `docs/ADR018-render-parity.md` reflect the updated `recover-postgres` behavior and body-gate coverage.
5. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 11 item 3), verified against `main@3854adf442`: `lego/backend/internal/postgres/rest.go:302-322` (`recovery field environmentId is unsupported`), `lego/backend/internal/postgres/recovery.go:438`, `lego/backend/internal/api/render_declared_fields_test.go:61-74` (6 ops probed, web PATCH only), and `lego/backend/internal/api/render_gated_bodies_test.go:38-78` (28 ops covered).
- **Goal linkage:** Advances Render API parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) and closes the `recover-postgres` `environmentId` refusal recorded at [ADR018 line 597](../../../docs/ADR018-render-parity.md#L597).
- **Expected outcome:** Every served Render JSON-body operation is mechanically guarded against schema-vs-struct drift and validator/decoder deadlocks, and Postgres PITR recovery honors Render's `environmentId`.
- **Why now:** ~30 body operations landed after `w5/m146` and `w8/137` without joining either guard, recreating the exact class of deadlocks `w8/135` and `w9/m472` hit.
- **Render parity:** Included (`t005`). Adds `environmentId` to Postgres recovery across REST/GraphQL/MCP and aligns all served REST JSON-body contracts.
