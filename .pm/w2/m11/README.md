# w2 · m11 — Unify service creation through the control-plane store

**Worker:** worker2 **Goal:** every create path (REST/GraphQL/MCP) produces a store-managed app, so private-repo builds always get a clone secret and deploy history always populates. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                       | est | depends_on         |
| ---- | ---------------------------------------------------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | Design: route all public-surface creates (REST/GraphQL/MCP) through the internal store path instead of writing the App CR directly, per ADR003 | 45m | —                    |
| t002 | Implement: `POST /v1/services` / GraphQL `createService` / MCP `create_web_service`+`deploy` write store rows (tenant + store app-id label) instead of the CR directly | 1h  | t001                 |
| t003 | Fix the projector (`projectSpec`) to mint an installation token / `spec.cloneSecret` when a row's repo matches a workspace GitHub connection | 45m | t001                 |
| t004 | Back-compat: confirm pre-existing directly-CR-created apps aren't broken by the routing change (backfill store rows or document as a legacy no-history class) | 30m | t002                 |
| t005 | Live acceptance: create a private-repo service via each of REST/GraphQL/MCP on the mock cluster; confirm build succeeds and deploy history populates | 45m | t002,t003,t004       |
| t006 | Render parity — verify create semantics + deploy-history behavior are consistent across REST/GraphQL/MCP/UI vs render.com    | 30m | t005                 |
| t007 | Simplify — `/simplify` over the code this milestone changed                                                                  | 15m | t006                 |
| t008 | Test coverage — meaningful tests for the unified create path + projector clone-secret minting                                | 30m | t006                 |
| t009 | Closeout — verify DoD met, then move the milestone to `done/`                                                                | 10m | t007,t008            |

## Definition of done

Creating a private-repo service via any of REST, GraphQL, or MCP builds successfully (clone secret present) and its deploys appear in the deploys API; no regression for public-image or existing creates.

## Source + Goal linkage

- **Source:** `w2/005.md`, filed during `w2/m8`+`m9`'s live acceptance (2026-07-12, real GitHub App on the mock cluster); promoted via `/pm-brainstorm more` 2026-07-12; note moved to `done/`.
- **Goal linkage:** pillar 1 (Render-compatible REST/GraphQL/MCP) + `docs/ADR003-control-plane.md`'s "Postgres is the source of truth" direction — this closes the seam ADR003 anticipated but `m2`/`m4`/`m8`/`m9` left open.
- **Expected outcome:** every service, regardless of which surface created it, gets correct private-repo builds and populated deploy history — no more "internal store path" vs "public surface" divergence.
- **Why now:** discovered live during the `m8`/`m9` acceptance run 2026-07-12 as a concrete build failure (`could not read Username for 'https://github.com'`), not a cosmetic gap.
- **Render parity closing task: included** — both create semantics and deploy-history population are REST/GraphQL/MCP-facing.
