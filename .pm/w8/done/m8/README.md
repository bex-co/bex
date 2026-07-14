# w8 · m8 — Service display name (rename without breaking the immutable resource id)

**Worker:** worker8 **Goal:** let a service be renamed for humans without touching its immutable k8s resource identity **Status:** done

## Tasks (in order)

| id   | title                                                                                          | est | depends_on | status     |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- | ---------- |
| t001 | `AppSpec.DisplayName` field (types + deepcopy + CRD yaml regen)                                  | 45m | —          | — **DONE** |
| t002 | `SetDisplayName` verb in `internal/apps/service.go`, using the existing `AuthorizeApp` chokepoint | 45m | t001       | — **DONE** |
| t003 | REST PATCH support (`patchServiceRequest.displayName`, `rest.go:269-330`) + `toRenderService` field mapping | 1h  | t002       | — **DONE** |
| t004 | GraphQL mutation/field + MCP field (if apps expose MCP update fields)                             | 1h  | t002       | — **DONE** |
| t005 | Render parity: verify `displayName` shape/semantics consistent across REST/GraphQL/MCP + dashboard UI | 45m | t003, t004 | — **DONE** |
| t006 | Simplify                                                                                           | 30m | t005       | — **DONE** |
| t007 | Test coverage                                                                                      | 1h  | t005       | — **DONE** |
| t008 | Closeout                                                                                           | 15m | t007       | — **DONE** |

## Definition of done

An App's `displayName` can be set/changed independently of its immutable `name` via REST/GraphQL/MCP, and is returned on reads across all three surfaces; the underlying k8s object name, TLS cert-secret naming, and `Subdomain` fallback are untouched by a display-name change.

## Closeout evidence

- `App.spec.displayName` is optional in the generated CRD; `SetDisplayName` uses the shared `Service.patch`/`AuthorizeApp` path and changes no routing or runtime identity field.
- REST, GraphQL, and MCP set/read/clear round trips are covered by real adapter tests; authorization denial and immutable-name/subdomain/host/restart/URL invariants are asserted.
- The dashboard displays the mutable label with an immutable-id fallback, exposes the Settings edit control, and keeps destructive confirmation tied to the immutable id.
- Render parity and the deliberate `displayName` extension are documented in ADR006/ADR018; Render's REST renames top-level `name`, while its official MCP server has no rename tool.
- Manual simplify review found the shared patch helper to be the smallest implementation; duplicate mutation logic was avoided, generated GraphQL sources were made reproducible, and formatter-only churn was removed.
- Verification: backend `go test ./...`, `go build ./...`, and backend lint; operator `make test` and `make build`; dashboard lint/typecheck, 901 tests, and production build. Repository-wide Go lint remains blocked only by 15 pre-existing findings in untouched `cmd/pg-sni-proxy` files.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 — `docs/ADR018-render-parity.md` "Change instance plan / type" row note, *"Remaining PATCH fields (name, buildFilter) not editable"* (`buildFilter` is a separately-recorded bex non-goal, `w5/done/m13`, and is not part of this milestone). Code survey confirmed `App` embeds `metav1.ObjectMeta` directly (`lego/types/v1alpha1/app_types.go:507-512`) — no `DisplayName`/friendly-name field exists today, and the PATCH handler (`rest.go:269-330`) never accepts a name-like field. The closest existing pattern is `Subdomain` (`app_types.go:274-283`), which already separates the routing-facing slug from the immutable CR `Name` — `DisplayName` mirrors that split rather than attempting a true rename-with-recreate (which would be large and touch the operator/cert-manager/owned-resource migration). Originally proposed under `w2`; materialized under `w8` **per user direction**.
- **Goal linkage:** Render parity on the service-update surface.
- **Expected outcome:** users can relabel a service without recreating it or disturbing its URL/cert identity.
- **Why now:** small, self-contained, no CRD-immutability conflict since it avoids touching the real resource name. Render parity included — the new field must round-trip identically across REST/GraphQL/MCP and be reflected in dashboard settings.
