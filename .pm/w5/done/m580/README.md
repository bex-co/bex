# w5 · m580 — Served Render REST list filters honor OpenAPI array/query contracts across all 30 list operations

**Worker:** worker5 **Goal:** fix the verified multi-value query-parsing bugs across `list-notification-overrides`, `list-build-sources`, `list-job`, and `list-events`, audit the remaining unguarded list handlers, and expand `TestListOperationsConsumeAdmittedQueries` from 9 hardcoded routes to an exhaustive OpenAPI-driven census over all 36 served list operations. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Fix multi-value `ownerId`, `name`, `status`, and `type` query parsing on `list-notification-overrides`, `list-build-sources`, `list-job`, and `list-events` — **DONE** | 1h15m | — |
| [t002](done/t002.md) | Audit the remaining unguarded served list handlers against their pinned OpenAPI query parameters and fix any silent ignores or scalar/array mismatches — **DONE** | 1h15m | t001 |
| [t003](done/t003.md) | Replace the 9-operation hardcoded `roots` map in `TestListOperationsConsumeAdmittedQueries` with an exhaustive OpenAPI-driven census over all 36 served lists — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Add composed-server behavioral tests exercising comma-separated (`explode=false`) and repeated query forms across newly covered lists and update `ADR018` § List-query census — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 15m | t007 |

Total: **8 tasks, about 5h35m**.

## Definition of done

1. Through the composed REST server, `GET /v1/notification-settings/overrides?ownerId=tea-a,tea-b` and `?ownerId=tea-a&ownerId=tea-b` return overrides from both authorized workspaces; `GET /v1/build-sources?name=a,b` returns both named build sources (and keeps literal percent-encoded commas per w8/m55); `GET /v1/services/{id}/jobs?status=pending,running` returns matching jobs instead of `400 unknown status "pending,running"`; and `GET /v1/services/{id}/events?type=deploy_ended&type=server_failed` returns both event types instead of dropping the second.
2. Every other served list operation in the pinned OpenAPI document reads every admitted query parameter with the arity (`explode=false` comma-separated and repeated forms) Render declares, or explicitly records a non-goal omission in `listQueryOmissions` and `docs/ADR018-render-parity.md` (including `list-build-sources` `includePreviews`).
3. `TestListOperationsConsumeAdmittedQueries` (`lego/backend/internal/api/list_query_contract_test.go`) drives all 30 served list operations directly from the pinned OpenAPI document rather than a hardcoded 9-operation subset, failing if any new served list leaves an admitted query parameter unread.
4. `cd lego/backend && go test ./...` (including the real-DB suite) and `make lint` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity in backend for w5` (2026-11-23, round 11 item 1), verified against `main@3854adf442`: `lego/backend/internal/apps/rest.go:1910-1936`, `lego/backend/internal/apps/build_sources.go:836-850`, `lego/backend/internal/jobs/rest.go:87-99`, `lego/backend/internal/events/rest.go:567-570`, and `lego/backend/internal/api/list_query_contract_test.go:38-49`.
- **Goal linkage:** Advances Render API parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)) and closes gaps under [ADR018 § List-query census](../../../../docs/ADR018-render-parity.md#list-query-census-w4164-2026-09-27).
- **Expected outcome:** OpenAPI-generated Render clients, SDKs, Terraform provider, and CLI callers get accurate filtered list responses on every served Render list endpoint, and future list endpoints cannot silently ignore admitted query parameters.
- **Why now:** 21 served list routes landed after `w4/164` without joining the 9-operation guard, leaving silent filter drops and 400s in production ahead of the `w4/722` OpenAPI re-pin.
- **Render parity:** Included (`t005`). Changes REST list-filter parsing across multiple served Render operations and updates `ADR018`.
