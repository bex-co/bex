# w8 · m22 — Generate Blueprint: export existing resources as render.yaml

**Worker:** worker8 **Goal:** Render's "Generate Blueprint" has a bex equivalent — select existing workspace services/datastores and get a generated render.yaml (env var **names only**, secrets emitted as `sync: false`, `fromService`/`fromDatabase` links where derivable) over REST/GraphQL/MCP plus a dashboard entry point, giving existing click-ops users a one-step path into IaC. **Status:** done

## Tasks (in order)

| id   | title                                                                       | est | depends_on |
| ---- | ---------------------------------------------------------------------------- | --- | ---------- |
| t001 | Core generator: workspace resources → render.yaml manifest — **DONE**                   | 60m | —          |
| t002 | Expose `generate` across REST/GraphQL/MCP — **DONE**                                    | 30m | t001       |
| t003 | Dashboard entry: select resources → download render.yaml — **DONE**                     | 45m | t002       |
| t004 | Render parity check (generated shape vs Render's export; round-trip surfaces) — **DONE** | 30m | t003       |
| t005 | Simplify (`/simplify` over the changed code) — **DONE**                                 | 30m | t004       |
| t006 | Test coverage (round-trip generate→validate, secret-absence, link derivation) — **DONE** | 45m | t004       |
| t007 | Closeout — **DONE**                                                                     | 15m | t006       |

## Definition of done

Selecting a deployed web service + Postgres + Key Value in the dashboard downloads a render.yaml that (a) passes bex's own `POST /v1/blueprints/validate` with zero errors, (b) contains no secret values anywhere — plain env vars keep values, secret-backed ones emit `sync: false`, datastore links emit `fromDatabase`/`fromService`, and (c) when created as a new blueprint against a repo, adopts the same resources with a no-op plan (adoption-by-name proven round trip). The same manifest is returned by the REST/GraphQL/MCP generate verbs. Fields bex round-trips are exactly the registry's `translated`/`equivalent` set — nothing emitted that bex's own validator would reject.

## Source + Goal linkage

- **Source:** blueprint lifecycle-semantics verification 2026-08-16, item 4: no export/generate capability exists in either layer (grep of `lego/backend/internal/apps/` blueprint routes and `dashboard/src/features/blueprints/` — routes are validate/create/list/get/sync/list-syncs/update/disconnect/deploy/preview only). Render's 2024 "Generate Blueprint" (select services → download render.yaml, env names only, secrets `sync: false`) is the reference. Building blocks exist: Render-compatible outbound serialization (`render.go`), the ADR049 compiler as the round-trip checker, the m19-swept capability registry as the emit-allowlist.
- **Goal linkage:** Render parity (ADR018 Blueprint row) + the IaC adoption funnel — the missing bridge from click-ops resources to blueprint-managed ones.
- **Expected outcome:** existing workspaces can adopt IaC in one step; the generated file is guaranteed-valid against bex's own contract.
- **Why now:** m19's registry sweep just defined exactly which fields bex truthfully supports — the generator can emit against that allowlist instead of guessing; adoption-by-name (verified working) makes the round trip a no-op plan, so the feature is mostly serialization. Render parity task included: new verb across REST/GraphQL/MCP + dashboard UI.
- **DO_NOT_DO constraints honored:** export only — no preview environments, disks, or region fields are ever emitted (they're not in the translated set); no new CLI work.
