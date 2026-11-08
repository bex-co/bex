# w5 · m283 — MCP Blueprint tools match Render's: create_blueprint plans, approve_blueprint applies

**Worker:** worker5 **Goal:** an agent that calls Render's Blueprint MCP tools on bex gets Render's semantics — `create_blueprint` plans without applying, `approve_blueprint` applies — with Render's tool names and arguments. **Status:** in progress — t001–t004 and t006–t008 done (shipped); t005 is the production MCP walk once the deploy is live, then t009

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Re-pin Render's MCP tool set and diff it — **DONE** | 40m | — |
| t002 | `create_blueprint` plans without applying, with Render's arguments — **DONE** | 1h | t001 |
| t003 | Add `plan_blueprint`, `approve_blueprint`, `get_blueprint`, `update_blueprint` — **DONE** | 1h20m | t002 |
| t004 | Retire or alias the bex-only planning tools and update the guards — **DONE** | 40m | t003 |
| t005 | Live walk on dev-5: create → plan → approve through MCP | 30m | t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006, t007 |
| t009 | Closeout | 10m | t008 |

Total: **9 tasks, about 5h55m**.

## Definition of done

- On MCP, `create_blueprint` (Render's arguments) creates a Blueprint and a plan but no resources; `approve_blueprint` then applies it, including `manualEnvVars`.
- `plan_blueprint`, `get_blueprint` and `update_blueprint` exist with Render's names, arguments and annotations; the pinned contract guard covers all five.
- No bex MCP tool shares a Render tool name with different semantics; ADR018 records the re-pin date and upstream commit.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 7 item 1 (2026-11-07); Render MCP server #205/#218/#221/#222/#224/#230.
- **Goal linkage:** Render parity first (ADR008 § Render parity first) and AI-native (ADR008): MCP is how agents drive bex.
- **Expected outcome:** an agent trained on Render's tools can onboard a Blueprint on bex without accidentally applying it.
- **Why now:** bex's `create_blueprint` currently applies the whole stack — the opposite of Render's same-named tool — so a Render-style call creates (and bills) resources. w2/m256 already built the planning verbs, so this is wiring plus a re-pin.
- **Render parity:** included (MCP surface change; REST/GraphQL consistency checked).

## Findings (2026-11-07, loopx w5)

- **t001 re-pin:** `scripts/render-mcp-drift.sh` against `render-oss/render-mcp-server` `main@16fe2e9` (2026-11-06) reported `added=[approve_blueprint create_blueprint get_blueprint plan_blueprint update_blueprint] removed=[none] changed=[none]` — only the five Blueprint tools moved. The pin now holds 28 tools; `renderMCPToolsSHA256` and the drift script's hash were updated together.
- **Upstream semantics (`pkg/blueprint/tools.go`):** each tool wraps Render's REST onboarding route. `create_blueprint` takes `repoUrl` (https:// or ssh://, no query or fragment; ssh becomes https), defaults `name` to the repository name, `path` to `render.yaml`, `autoSync` to true and **`existingResources` to `create_new`**. `update_blueprint` returns `{blueprint, planningOutcome, sync, notices, error}`: it skips the write when nothing changed and plans only when the branch or path changed.
- **t002–t004 decisions:** the five tools run the same service verbs as REST (`CreatePlannedBlueprint`, `PlanBlueprint`, `ApproveBlueprint`, `GetBlueprintByID`, `UpdateBlueprint`). bex's old apply-at-once `create_blueprint` and the w2/m256 extensions `create_planned_blueprint` and `plan_blueprint_sync` are removed rather than aliased: a Render-named tool with other semantics was the defect, and create → approve covers the old flow. MCP inventory: 1:1 10 → 15, Extension 204 → 198, total 225 → 224 (`TestMCPParityInventory`, ADR018 §MCP).
- **t006 Render parity:** REST already served Render's routes (w2/m256), and GraphQL keeps its bex-named onboarding verbs (Render has no GraphQL); there is no dashboard change. One surface difference is by design: MCP `create_blueprint` defaults to `create_new` as Render's MCP server does, while REST keeps Render's REST default (recorded in ADR049).
- **t007 Simplify:** one `toRenderBlueprint` and one `toBlueprintDetail` now serve REST and MCP; `update_blueprint` normalizes the path before comparing and checks the edit right even on a no-op. Not changed: a branch move fetches the file twice (once to validate the branch in `UpdateBlueprint`, once to plan), which is correct and rare.
- **t008 tests:** `TestRenderBlueprintMCPTools` covers repoUrl refusals, create planning without resources, the `create_new` default, get, approve with and without manual env vars, plan `no_changes`, and update (refusals, no-op, rename without a plan, a branch move that plans); there is also `TestParseBlueprintRepoURL`.
- **t005:** the dev-5 walk could not run: the shared CAPD cluster answered with a certificate the published kubeconfig does not trust (`x509: certificate signed by unknown authority`, even after republishing it). The walk moves to production MCP (`api.bex.co/mcp`) once this deploy is live.
