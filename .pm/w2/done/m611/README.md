# w2 · m611 — Build Source build logs and `build=bld` filtering across REST, GraphQL, MCP and live tail

**Worker:** worker2 **Goal:** Authorize `resource=bds-…` and map carrier deploy IDs (`dep-…`) ⇄ public build IDs (`bld-…`) in `lego/backend/internal/logs/` so Build Source builds have full historical logs, label values, and SSE/WebSocket live tail across REST, GraphQL, and MCP. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Authorize explicit `bds-…` resources in `logs/service.go` (`AuthorizeApp` / `AuthorizeAppFresh` with `core.WithBuildSources`) across history, label values, and live tail without leaking carriers into unscoped workspace/service-only queries — **DONE** | 50m | — |
| t002 | Map carrier deploy IDs (`dep-…`) ⇄ public build IDs (`bld-…`) in `logs/build_filter.go` and `logs/progress.go` so `build=bld-…` filters carrier build Jobs and platform narration and emitted `build` labels carry `bld-…` — **DONE** | 1h | t001 |
| t003 | Verify `DeployProgress` wiring in `api/server.go:1017` and carrier narration lines so Build Source builds narrate build-only phases (no phantom runtime rollout or port-scan lines) — **DONE** | 40m | t002 |
| t004 | Wire `bds-…` / `bld-…` support cleanly across REST (`/v1/logs`, `/v1/logs/values`, `/v1/logs/subscribe`), GraphQL (`logs`, `logLabelValues`, SSE stream), and MCP (`list_logs`, `list_log_label_values`) — **DONE** | 40m | t003 |
| t005 | Add unit and integration tests covering RBAC, `resource=bds-…` + `build=bld-…` history and live tail, `build=bld-…` label emission, and non-leakage into service-only queries — **DONE** | 50m | t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout — **DONE** | 10m | t007, t008 |

## Completion summary (2026-11-24)

- **Authorization & non-leakage (`logs/service.go`):** `logResourceCtx` applies `core.WithBuildSources(ctx)` only for explicit `bds-…` IDs across `authorizeLogOwner`, `Logs`, `QueryLogs`, `labelValues`, and `followLogs`; `revalidateTailTarget` enforces fresh `can_view_logs` + `NotFoundIfDeleting`. `scopeToBuildSource` defaults untyped `resource=bds-…` reads to `type=build` and returns empty for non-build-only types/filters; project-wide queries (`PGStore.ListProjectAppIDs`) keep Build Source carriers excluded and reject `project=…&resource=bds-…` with `400`.
- **Public `bld-…` mapping & build-only narration (`logs/build_filter.go`, `logs/progress.go`):** `publicBuildID` maps carrier `dep-…` rows to `bld-…` via `id.Rekind` across build-Job indexing, `build=bld-…` filtering (history, label values, and live tail), `deployId=bld-…` scoping, projected instance round-trips, and emitted `build` labels while leaving ordinary `srv-…` build logs unchanged. Git-backed Build Source narration emits only build-phase lines (`Build queued`, wait line, `Building from <repo>@<ref>`, `Build succeeded 🎉` / `Build canceled` / `Build failed`) and suppresses OIDC, pre-deploy, rollout, port-scan, and live-URL lines; image-backed Build Sources emit no narration.
- **Render parity note:** Per Render's pinned OpenAPI (`list-logs-values` 6-value enum: `host`, `instance`, `level`, `method`, `statusCode`, `type`), `GET /v1/logs/values?label=build` stays Render's `400` (`TestRenderRequestValidatorAdmitsTheLogsBuildFilter`), while `build=bld-…` filters `/v1/logs/values`, GraphQL `logLabelValues`, and MCP `list_log_label_values` across all six discoverable labels.
- **Verification & docs:** Unit, composed-server (`NewServer` + OpenAPI + Loki + REST/GraphQL/MCP + scoped keys), and real PostgreSQL + OpenFGA E2E suites plus `make lint` pass cleanly; `docs/ADR018/logs.md` and `docs/ADR018/services-lifecycle.md` updated.

## Definition of done

- `GET /v1/logs?resource=bds-…&build=bld-…`, `GET /v1/logs/values?resource=bds-…&label=build`, `GET /v1/logs/subscribe?resource=bds-…&build=bld-…`, GraphQL `logs(resource: "bds-…", build: ["bld-…"])` / `logValues`, and MCP `list_logs` / `list_log_label_values` all return the Build Source's build Job output and build-phase platform narration with `build=bld-…` labels (never `404` and never exposing internal `dep-…` IDs as the `build` label).
- Unscoped workspace-wide or project-wide service log queries that do not explicitly name a `bds-…` resource do not mix Build Source carrier logs into unrelated service views.
- Cross-workspace and unauthorized callers receive the exact same `404`/`403` contract as service logs; viewers with `can_view_logs` can read Build Source build logs.
- Build Source narration emits only build-phase events (no runtime rollout or port-scan narration lines).
- `cd lego/backend && go test ./...` and `make lint` pass, and `docs/ADR018/logs.md` + `docs/ADR018/services-lifecycle.md` reflect the shipped behavior.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w2` (2026-11-23), materialized via `/pmx w2`. Verified at `0f9166bc7f`: `lego/backend/internal/logs/service.go` calls `AuthorizeApp` / `AuthorizeAppFresh` without `core.WithBuildSources(ctx)` (returning `404` for every `bds-…`), and `lego/backend/internal/logs/build_filter.go:47-61` indexes build Jobs only by `dep-…`.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)) and [ADR097](../../../../docs/ADR097-build-sources.md) — Render's OpenAPI documents `GET /v1/logs` `build` for build IDs alongside `resource` and `type=build`. Today users cannot inspect a failed Build Source build's logs on any API, MCP, or dashboard surface.
- **Expected outcome:** Complete historical and real-time build log observability for Build Sources across REST, GraphQL, and MCP.
- **Why now:** Required backend foundation before `w2/m614` can stream live Build Source build logs in the Dashboard.
- **Render parity:** Included — extends REST, GraphQL, and MCP log query, label-value, and live-tail surfaces to Build Sources (`bds-…`) and public build IDs (`bld-…`).
- **Brainstorm recommendations, not user decisions:** Under ADR008 Render parity first, Build Source build logs emit `type=build` and `build=bld-…` (mapping carrier `dep-…` ⇄ `bld-…` via `id.Rekind`), and explicit `resource=bds-…` queries are authorized via `can_view_logs` while unscoped service queries stay service-only.
