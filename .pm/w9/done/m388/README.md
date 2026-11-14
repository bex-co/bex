# w9 · m388 — Workspace-level inbound IP rules, as on Render Scale and Enterprise

**Worker:** worker9 **Goal:** a Scale or Enterprise workspace's Admin sets inbound IP rules for the whole workspace; they apply to every web service, static site, Postgres and Key Value in it, on top of environment and service rules (a source must pass every layer), and `GET /v1/owners` returns them as Render's `ipAllowList`. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | ADR032: the workspace layer replaces the deferral; ADR018 gains the row — **DONE** | 45m | — |
| [t002](done/t002.md) | Store and core: workspace rules, Admin-only, Scale-gated, seeded allow-all — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Types and operator: a workspace-layer field on App, Database and KeyValue, chained as a third HTTP and SNI middleware — **DONE** | 150m | t002 |
| [t004](done/t004.md) | Fan-out: a rule change reaches every eligible resource; new resources inherit it — **DONE** | 90m | t003 |
| [t005](done/t005.md) | REST, GraphQL and MCP: `ipAllowList` on owners, a GraphQL setter, and an audit event — **DONE** | 90m | t004 |
| [t006](done/t006.md) | Dashboard: Workspace Settings → Inbound IP Rules — **DONE** | 90m | t005 |
| [t007](done/t007.md) | Live check on production: a workspace rule refuses a source that environment and service rules allow — **DONE** | 60m | t006 |
| [t008](done/t008.md) | Render parity: every surface this milestone touched agrees, and matches render.com — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 45m | t008, t009 |
| [t011](done/t011.md) | Closeout — **DONE** | 10m | t010 |

## Definition of done

On production (QA user, Scale workspace):

- An Admin sets workspace rules in Workspace Settings; a source allowed by environment and service rules but not by the workspace rules is refused by a web service (HTTP) and a public Postgres (SNI); removing the rule restores access.
- A Developer, or a Hobby/Pro workspace, is refused with one machine code on GraphQL and an explanation in the dashboard.
- `GET /v1/owners` and `GET /v1/owners/{ownerId}` return `ipAllowList` in Render's shape; MCP shows it wherever the owner is read.
- Each change writes an audit entry; ADR032 and ADR018 describe the layer.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w9` (2026-11-13), item 4, filed by `/pmx` 2026-11-13.
  - Render: "Scale and Enterprise orgs can also set rules for … An entire workspace", "an inbound IP must be allowed by each level's rules", Admin-only for the workspace ([Inbound IP rules](https://render.com/docs/inbound-ip-rules), checked 2026-11-13). The pinned `owner` schema carries `ipAllowList` (`lego/backend/internal/api/openapi/render-public-api-1.json`); owners are read-only on Render's REST.
  - bex: `renderOwner` omits `ipAllowList` (`lego/backend/internal/workspaces/render.go:38-39`); ADR032:210 records "Workspace layer: explicit deferral", a w4/m32 scope cut (`.pm/w4/done/m32/t003.md:44`), not an ADR018 sign-off. The environment layer it would join: `core.EnvironmentLayerCIDRs` (`lego/backend/internal/core/cidr.go:333-339`), CRD `EnvironmentIPAllowList` (`lego/types/v1alpha1/app_types.go:998-1009` and the Database/KeyValue twins), enforced in `lego/operator/internal/controller/app_controller.go` and `lego/operator/internal/sniproxy/allowlist.go`; plan gate `FeatureInboundIPRules` at Scale (`lego/backend/internal/core/plan_gate.go:54`).
- **Goal linkage:** ADR008 Render parity first; tenant network isolation (ADR043).
- **Expected outcome:** a Scale workspace locks every public entry point to, say, an office CIDR in one place.
- **Why now:** the middleware chain was built layer-count-agnostic (ADR032:210), so the layer is cheap to add; under the parity rule an unsigned deferral is amended rather than kept, and ADR018 has no row for it today.
- **Render parity:** included (t008), because the change touches REST, GraphQL, MCP and the dashboard.
