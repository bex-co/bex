# w8 · m8 — Service display name (rename without breaking the immutable resource id)

**Worker:** worker8 **Goal:** let a service be renamed for humans without touching its immutable k8s resource identity **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | `AppSpec.DisplayName` field (types + deepcopy + CRD yaml regen)                                  | 45m | —          |
| t002 | `SetDisplayName` verb in `internal/apps/service.go`, using the existing `AuthorizeApp` chokepoint | 45m | t001       |
| t003 | REST PATCH support (`patchServiceRequest.displayName`, `rest.go:269-330`) + `toRenderService` field mapping | 1h  | t002       |
| t004 | GraphQL mutation/field + MCP field (if apps expose MCP update fields)                             | 1h  | t002       |
| t005 | Render parity: verify `displayName` shape/semantics consistent across REST/GraphQL/MCP + dashboard UI | 45m | t003, t004 |
| t006 | Simplify                                                                                           | 30m | t005       |
| t007 | Test coverage                                                                                      | 1h  | t005       |
| t008 | Closeout                                                                                           | 15m | t007       |

## Definition of done

An App's `displayName` can be set/changed independently of its immutable `name` via REST/GraphQL/MCP, and is returned on reads across all three surfaces; the underlying k8s object name, TLS cert-secret naming, and `Subdomain` fallback are untouched by a display-name change.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 — `docs/ADR018-render-parity.md` "Change instance plan / type" row note, *"Remaining PATCH fields (name, buildFilter) not editable"* (`buildFilter` is a separately-recorded bex non-goal, `w5/done/m13`, and is not part of this milestone). Code survey confirmed `App` embeds `metav1.ObjectMeta` directly (`lego/types/v1alpha1/app_types.go:507-512`) — no `DisplayName`/friendly-name field exists today, and the PATCH handler (`rest.go:269-330`) never accepts a name-like field. The closest existing pattern is `Subdomain` (`app_types.go:274-283`), which already separates the routing-facing slug from the immutable CR `Name` — `DisplayName` mirrors that split rather than attempting a true rename-with-recreate (which would be large and touch the operator/cert-manager/owned-resource migration). Originally proposed under `w2`; materialized under `w8` **per user direction**.
- **Goal linkage:** Render parity on the service-update surface.
- **Expected outcome:** users can relabel a service without recreating it or disturbing its URL/cert identity.
- **Why now:** small, self-contained, no CRD-immutability conflict since it avoids touching the real resource name. Render parity included — the new field must round-trip identically across REST/GraphQL/MCP and be reflected in dashboard settings.
