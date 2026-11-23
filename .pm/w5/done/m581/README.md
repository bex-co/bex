# w5 · m581 — Close the Render REST response-conformance blind spot across all served operation families

**Worker:** worker5 **Goal:** fix `DELETE /v1/build-sources/{buildSourceId}`'s HTTP 204 JSON-body defect, enforce a 204-no-body check in `rendertest`, and expand Render REST response-conformance coverage beyond the current 34/219 operations in `TestRenderConformance` so every served non-streaming Render operation family is exercised and schema-validated without growing `rendertest.Allowlist`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Fix `DELETE /v1/build-sources/{buildSourceId}` to use `core.HandleNoBody(http.StatusNoContent, …)` and reject non-empty bodies or `Content-Type` on HTTP 204 in `rendertest.Validate` — **DONE** | 45m | — |
| [t002](done/t002.md) | Add `TestRenderConformance` coverage for build sources, disks + disk snapshots, blueprints, webhooks, and registry credentials (reads + writes/deletes) and fix any violations uncovered — **DONE** | 1h45m | t001 |
| [t003](done/t003.md) | Add `TestRenderConformance` coverage for owners/members/users, workspace + org audit logs, notification settings/overrides, jobs, static routes/headers, metrics, and sandbox groups/builds/snapshots/execs, fixing any violations uncovered — **DONE** | 1h45m | t001 |
| [t004](done/t004.md) | Add an inventory guard verifying every served non-streaming Render operation family is exercised by response-conformance validation and update `ADR018` — **DONE** | 45m | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 15m | t007 |

Total: **8 tasks, about 6h20m**.

## Definition of done

1. `DELETE /v1/build-sources/{buildSourceId}` (`lego/backend/internal/apps/build_sources.go:861`) uses `core.HandleNoBody(http.StatusNoContent, …)` and answers HTTP 204 with an empty body and no `Content-Type` header; `rendertest.Validate` fails any 204 response that writes a body or sets `Content-Type`.
2. Every served non-streaming Render REST operation family (build sources, disks & disk snapshots, blueprints, webhooks, registry credentials, owners/members/users, audit logs, notification settings/overrides, jobs, static routes/headers, metrics, sandbox groups/builds/snapshots/execs, plus the existing core families) is exercised against the pinned OpenAPI response schemas and documented status codes in `internal/api` conformance tests.
3. Every response-schema or status-code defect uncovered across those families is fixed in production handlers; `rendertest.Allowlist` stays at ≤ 23 entries (`TestAllowlistOnlyShrinks` passes with zero new entries added).
4. An automated inventory guard fails if a new served non-streaming Render operation family is added without response-conformance coverage.
5. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 11 item 2), verified against `main@3854adf442`: `lego/backend/internal/apps/build_sources.go:861` (the sole `core.HandleJSON(http.StatusNoContent, …)` call in the repo, with zero REST tests hitting it), `lego/backend/internal/rendertest/rendertest.go:199-203` (does not reject bodies on 204), and `lego/backend/internal/api/conformance_test.go:263-530` (explicitly tests only 34 of 219 served Render operations).
- **Goal linkage:** Advances Render API parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) and hardens [ADR018 § Allowlisted response-schema divergences](../../../../docs/ADR018-render-parity.md#allowlisted-response-schema-divergences-w3m272).
- **Expected outcome:** Every served Render REST family is continuously verified for status-code and response-schema parity, eliminating silent wire drift on the ~85% of operations outside `TestRenderConformance`.
- **Why now:** Closing the conformance blind spot ahead of `w4/722` (the 2026-11 OpenAPI re-pin) ensures upstream response schema changes are caught across every served family rather than only the 34 core operations.
- **Render parity:** Included (`t005`). Fixes REST response status/body behavior and updates `ADR018`.
