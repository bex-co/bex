# w1 · m46 — Service URLs speak `srv-` ids: close the GraphQL name-as-id deviation

**Worker:** worker1 **Goal:** the dashboard addresses services by their minted opaque id — `/services/srv-…/metrics`, matching Render's `dashboard.render.com/web/srv-…/deploys` — instead of the App name, closing ADR020's "GraphQL `Service.id` returns the App name" deviation while name-based URLs keep resolving through the existing fallback. **Status:** done (2026-07-16)

## Tasks (in order)

| id   | title                                                                     | est | depends_on             |
| ---- | ------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Flip GraphQL `Service.id` to the minted `srv-` id + verb round-trip audit — **DONE** | 40m | —                      |
| t002 | Dashboard sweep: id-driven URLs, id≠name assumptions, alias consistency — **DONE** | 50m | t001                   |
| t003 | Name-fallback compat pinned: legacy URLs and label-less CRs still resolve — **DONE** | 30m | t001                   |
| t004 | Docs: close the ADR020 deviation, sweep ADR018 + dashboard-routes.md — **DONE** | 20m | t002, t003             |
| t005 | Render parity check across REST/GraphQL/MCP/UI — **DONE** | 30m | t004                   |
| t006 | Simplify pass over the milestone's changes — **DONE** | 30m | t005                   |
| t007 | Test coverage for the shipped behavior — **DONE** | 40m | t005                   |
| t008 | Closeout — **DONE** | 20m | t006, t007             |

## Definition of done

On dev-1 (which holds the `whoami` service with a real minted `srv-` id): the services list links to `/services/srv-…`, every detail tab loads under that URL with all GraphQL calls round-tripping the `srv-` id, the header's "Service ID:" shows the `srv-` id (Render's own display) with the name still shown as the slug/title, the m45 alias `/web/{srv-id}` deep-links land id-consistently end-to-end, and the OLD name-shaped URL `/services/whoami/...` still resolves to the same service (fallback). A legacy App without `LabelAppID` keeps working with its name as id. `go test ./...`, dashboard suites, and lint stay green; ADR020's deviation entry is closed.

**Verified live on dev-1, 2026-07-16**: the overview links `whoami` to `/services/srv-d9crtpi9086g5in0ftkg`; every tab loads under the id URL; the header shows "Service ID: srv-…" with the name as the title (a layout-level effect swaps each tab's param-templated document title for the resolved name — "whoami · Metrics", the project-sidebar precedent); Render's exact `/web/srv-…/metrics` shape lands end-to-end through the m45 alias; the old `/services/whoami/logs` name URL still resolves. The flip needed ZERO verb-layer changes — `AuthorizeApp`'s LabelAppID-first seam already accepted both shapes — so the entire backend diff is one resolver line plus tests (5 new: emit, dual-accept, legacy fallback, list, and a REST=GraphQL=core id-equality pin). Incidental closeout: the milestone surfaced a **migration-number collision** — my m33 `0040_audit_member_roles` vs a concurrent session's committed `0040_deploy_commit_author_at` (the guard test had main's backend CI red) — resolved by renumbering mine to 0041 with `IF NOT EXISTS` idempotence (prod applied theirs as 40; dev-N environments that applied mine as 40 no-op replay it), and dev-1's schema_migrations was rolled 40→39→41 to apply both. `services.$serviceId`'s literal-pattern `requireAuth("/services/$serviceId")` also became no-arg (its `next` was a literal `$serviceId` string). Suites: backend full + real-Postgres chain green, 1306 dashboard tests + lint green, golangci 0 issues; `/simplify` applied (reused `mustSchema`, strict `renderService` decode in the parity pin, one stale "(bex App name)" MCP description fixed).

## Source + Goal linkage

- **Source:** user request 2026-07-16 — "use `srv-****` instead of `/services/node-hello-world/metrics` … consistent with `dashboard.render.com/web/srv-…/deploys`" — plus this session's scoping: `internal/apps/graphql.go:256` is the one resolver returning `a.Name`; `AppView.ID` already carries the minted id (`publicID`, legacy fallback to name); `core.Base.AuthorizeApp`/`GetApp` already resolve `LabelAppID` (srv-) first with `LabelServiceName` fallback, so every GraphQL verb already **accepts** both shapes — only the emitted `id` lags. ADR020 § Known deviations records the deviation as deliberately out of w2/m43's scope.
- **Goal linkage:** Render parity (docs/ADR018-render-parity.md; docs/ADR020-identifiers.md) — REST and MCP already speak `srv-`; GraphQL and the dashboard are the last name-as-id service surfaces, and URLs are the habit surface m45 just finished aligning.
- **Expected outcome:** service URLs and the GraphQL id become stable opaque ids that survive rename (the same property w9/m3 and w9/m6 gave Postgres and Key Value), byte-consistent with what Render's CLI and API mint.
- **Why now:** straight user follow-up to m45's route-parity walk — the live comparison put `/services/node-hello-world/metrics` next to `/web/srv-…/deploys`; the resolution seam already accepts srv- ids, so the change is a controlled flip plus a sweep, not a re-architecture. Render parity closing task included: the change touches GraphQL + UI (REST/MCP asserted unchanged).
- **Adjacent deviation deliberately not fixed here:** GraphQL `Domain.id` also returns the name (`graphql.go:516`) while domains mint `cdm-` ids — filed as `w1/028`, not silently absorbed.
