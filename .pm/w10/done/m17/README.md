# w10 · m17 — Simplify agent-session creation: task + repo + agent; a workspace-level agent environment

**Worker:** worker10 **Goal:** Starting an agent session asks for exactly what Devin, Codex cloud, Claude Code on the web and Copilot ask for: the task, an optional repository and the agent. The per-session Advanced panel is gone. The branch is always derived. The model and the sandbox's network access are configured once per workspace under Settings → Agents, with network tiers. Package registries are reachable for the whole session, so agents can build and test. **Status:** done (2026-10-28 — shipped `83ecf0502`, `953642326`, `fa23545e5`, `358cef5ed`, `dfbb006c1`; verified in production, see done/t007.md)

## Tasks (in order)

| id   | title                                                                                         | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Composer: remove the Advanced panel; branch always derived; show branch + PR after creation — **DONE**   | 45m  | —          |
| t002 | Model override cannot break the platform route (pinned model wins; no 403 footgun) — **DONE**            | 30m  | —          |
| t003 | Workspace agent environment: network tier (registries / full / custom) + default model, API — **DONE**  | 2h   | —          |
| t004 | Sandbox egress follows the workspace tier; package registries stay open in the agent phase — **DONE**    | 1.5h | t003       |
| t005 | Settings → Agents UI for the environment (tier picker, custom allowlist, default model) — **DONE**      | 1.5h | t003       |
| t006 | Docs: ADR047 D5 amendment (registries through the agent phase), ADR090, AGENTS env tables — **DONE**     | 30m  | t004       |
| t007 | Verify in production: create from the simplified composer; an agent runs `go build`/`yarn test` — **DONE** | 30m  | t001, t004, t005 |
| t008 | Render parity (REST/GraphQL/MCP environment + session fields, dashboard + mobile consistency) — **DONE** | 20m  | t007       |
| t009 | Simplify — **DONE**                                                                                      | 20m  | t008       |
| t010 | Test coverage — **DONE**                                                                                 | 30m  | t008       |
| t011 | Closeout — **DONE**                                                                                      | 10m  | t009, t010 |

## Definition of done

- The new-session composer at `https://dashboard.bex.co/agents` shows only the task box, a repo picker and the agent selector. There are no branch, model or hostname fields. A created repo session shows its derived `bex-agent/<slug>` branch and PR in the header.
- A workspace has one agent environment: network tier `registries` (default) | `full` | `custom` (≤32 hostnames), plus a default model. It can be read and written over REST, GraphQL and MCP, and set under Settings → Agents.
- With the default tier, a live repo session can reach the built-in package registries (npm, PyPI, Go proxy, Maven, crates, RubyGems, Docker Hub) **during the agent phase**: a production session runs `go build ./...` or `yarn install && yarn test` successfully. Model-provider hosts stay proxy-only. The cluster admission policy (`bex-api-session-egress`) admits the new policy shape.
- For a platform-route workspace, a per-session model value that differs from the pinned model is ignored, or rejected at create time with a typed error. It never reaches the gateway as a 403.
- The session REST/GraphQL/MCP `branch`/`model`/`egressAllowlist` inputs keep working, so the API is unchanged for CLI/MCP users. The dashboard no longer sends them.
- The backend real-DB suite, `make lint`, the dashboard and mobile suites and Prettier are all green.

## Source + Goal linkage

- **Source:** user report 2026-10-27: "https://dashboard.bex.co/agents 似乎过于复杂 … 这些似乎 devin 都没有，如何简化？" The read-only review found:
  - The Advanced panel in `dashboard/src/features/agent-sessions/components/new-session-composer.tsx` (`ConfigurationFields`) exposes three fields.
    - **branch:** auto-derived by `deriveBranch` anyway.
    - **model override:** conflicts with the platform route's gateway model pin, which 403s.
    - **per-session extra egress hostnames:** exposed to the user even though the platform should get this right by default.
  - `sessionegress/policy.go` `TransitionToAgent` closes package registries after setup. The `/rt-simplify-logic` session `ags-dbf8nggkbcjs73a5r3u0` reported it "can't run go build/go test/make lint/yarn test" (no egress, empty Go module cache) and could only verify by inspection.
  - Prior art:
    - Devin: one task box; machine/network config at org/repo level.
    - Codex cloud: per-environment internet access (off / common-dependencies preset / full / custom allowlist), chosen once per environment.
    - Claude Code on the web: environment network access, limited (package registries) or full.
    - Copilot: repo-level firewall allowlist that permits common registries by default.
- **Goal linkage:** ADR008 AI-native pillar; ADR047 agent sessions (D5 egress).
- **Expected outcome:**
  - Creating a session takes one field.
  - Agents can build and test the code they change, so PR quality rises.
  - The model and network are configured once per workspace instead of per session.
- **Why now:** the composer is the first impression of the agent product. The agent-phase registry lockout is measurably degrading delivered PRs (they ship unverified), and the model-override footgun can break platform-route sessions.
- **Security trade-off (user direction 2026-10-27 to proceed with the simplification):**
  - Opening registries in the agent phase widens exfiltration paths beyond the setup phase, through package uploads to public registries.
  - It is accepted because the model key is a placeholder and Git push is branch-pinned, and this matches every compared product's default.
  - The `full` tier is opt-in per workspace.
  - t006 records it in ADR047 D5.
- **Render parity included (t008):** new workspace environment fields on REST/GraphQL/MCP plus UI changes. There is no render.com equivalent, so this is a cross-surface consistency check.
