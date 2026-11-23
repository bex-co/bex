# w5 · m583 — Render `errorCode` vocabulary parity on served REST refusals, guarded for all 33 pinned codes

**Worker:** worker5 **Goal:** emit Render's pinned machine-readable `errorCode` values where bex already enforces the corresponding refusal (starting with `too_many_resources` on `GET /v1/metrics/bandwidth-sources` and `build_source_region_mismatch` on `POST /v1/services` and `PATCH /v1/services/{id}`) and add an exhaustive guard classifying every value in `components.schemas.errorCode`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Emit Render's `"code": "too_many_resources"` on `GET /v1/metrics/bandwidth-sources` instead of a codeless `WriteErrStatus` string prefix | 45m | — |
| [t002](t002.md) | Audit the remaining 11 unemitted pinned `errorCode` values and enforce `build_source_region_mismatch` when linking a git-backed build source whose declared Render region differs from the service's | 1h15m | t001 |
| [t003](t003.md) | Add `TestEveryPinnedRenderErrorCodeIsClassified` covering all 33 values in `components.schemas.errorCode` and update `ADR018` § API contract | 45m | t002 |
| [t004](t004.md) | Render parity | 20m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 15m | t006 |

Total: **7 tasks, about 4h05m**.

## Definition of done

1. `GET /v1/metrics/bandwidth-sources` with > 30 resources answers HTTP 400 with `"code": "too_many_resources"` through the composed server (`withRenderErrorCodes` preserves it).
2. `POST /v1/services` and `PATCH /v1/services/{id}` linking a git-backed build source whose declared Render region differs from the service's declared Render region answer HTTP 400 with `"code": "build_source_region_mismatch"` across REST, GraphQL, and MCP.
3. `TestEveryPinnedRenderErrorCodeIsClassified` accounts for every value of `components.schemas.errorCode` (33 values at the current pin), requiring each to be either emitted by a tested handler or explicitly classified with its reason (e.g., Stripe preauth codes, cursor-origin receipts, runtime-snapshot codes pending `w2/m322`).
4. `docs/ADR018-render-parity.md` § API contract records the emitted codes and guard.
5. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 11 item 4), verified against `main@3854adf442`: `lego/backend/internal/metrics/bandwidth_sources.go:46-47,97-101` (writes `"too_many_resources: …"` via codeless `core.WriteErrStatus`), `lego/backend/internal/api/openapi/render-public-api-1.json` (`components.schemas.errorCode` 33 values; `POST /services` and `PATCH /services/{serviceId}` document `400` `build_source_region_mismatch`), and `lego/backend/internal/apps/build_sources.go`.
- **Goal linkage:** Advances Render API parity ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first)) and completes the companion half of [ADR018 § API contract — Error body `code`](../../../docs/ADR018-render-parity.md#api-contract) (`w8/411`).
- **Expected outcome:** Render SDKs and API clients receive exact machine-readable `errorCode` values instead of missing `code` fields on served refusals, and future OpenAPI re-pins cannot silently add unhandled error codes.
- **Why now:** `w8/411` just stripped non-Render codes from served REST error bodies; the companion gap is ensuring Render's actual documented codes are emitted structurally where bex enforces the condition.
- **Open decision note (recommendation, not a user decision):** Enforcing `build_source_region_mismatch` by comparing declared Render regions (`w9/105`) on git-backed build sources even though bex places resources in a single `BEX_REGION` is the proposal's recommendation under `ADR008 § Render parity first` (matching Render's OpenAPI contract for `POST /services` and `PATCH /services/{serviceId}`); any divergence to skip region-mismatch checking on single-region bex would require explicit user sign-off in `ADR018`.
- **Render parity:** Included (`t004`). Changes REST/GraphQL/MCP error codes and updates `ADR018`.
