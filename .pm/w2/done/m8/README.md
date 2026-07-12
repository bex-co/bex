# w2 · m8 — Connect GitHub: GitHub App connection + repo listing

**Worker:** worker2 **Goal:** an admin connects a GitHub account once (bex GitHub App install); agents and the dashboard can enumerate its repos on all four surfaces. **Status:** **DONE 2026-07-12** — backend shipped 2026-07-11; t006 dashboard card re-landed 2026-07-12 after the codegen fix (duplicate `Workspaces` op deduped into workspaces.graphql; offline `SCHEMA_JSON` regen; 634/634 dashboard tests + typecheck + `yarn build` green); t010 live DoD PASSED 2026-07-12 (evidence below).

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | ADR `docs/ADR026-github-integration.md` — GitHub App model, manifest flow, env vars, trust chain | 40m | —          | — **DONE** |
| t002 | `lego/backend/internal/github` client — app JWT, installation tokens, repo listing        | 45m | t001       | — **DONE** |
| t003 | Control-plane store: `git_connections` table + connection verbs (authz-gated)             | 40m | t002       | — **DONE** |
| t004 | REST: connect · callback · get/delete connection · `GET /v1/repos`                        | 45m | t003       | — **DONE** |
| t005 | GraphQL (`gitConnection`/`repos` + mutations) and MCP (`list_repos`, `get_git_connection`) | 40m | t004       | — **DONE** |
| t006 | Dashboard: Settings → "Connect GitHub" card (install link, status, disconnect)            | 45m | t005       | — **DONE** (reverted in `7504210`, re-landed 2026-07-12 after the codegen fix; verified live against the mock cluster — card shows/creates/removes, screenshot `.playwright-mcp/m8-connect-github-card-connected.png`) |
| t007 | Render parity — cross-surface consistency + declare REST/MCP repo surface a bex superset  | 30m | t006       | — **DONE** |
| t008 | Simplify — `/simplify` over the milestone's diff                                          | 30m | t007       | — **DONE** (run jointly with m9/t007 over the combined diff) |
| t009 | Test coverage — connection lifecycle, 503-when-unconfigured, token minting, pagination    | 40m | t007       | — **DONE** |
| t010 | Closeout — DoD verified, move to `done/`                                                  | 15m | t009       | — **DONE** (live DoD PASSED 2026-07-12, evidence below) |

## Definition of done

With `BEX_GITHUB_APP_ID`/`BEX_GITHUB_APP_PRIVATE_KEY`/`BEX_GITHUB_APP_SLUG` + `BEX_CP_DB_URI` set, connecting a real GitHub account (install → callback) makes `GET /v1/repos`, GraphQL `repos`, and MCP `list_repos` return that installation's repos (private included); disconnect empties them; any GitHub env var unset ⇒ every git-connect verb returns 503; the dashboard Settings card shows, creates, and removes the connection. Verified live against a real GitHub account.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-11 (GitHub-connect topic, Render docs `render.com/docs/github` + `/web/new` flow); closes the connect+list half of [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md) row "Git connections (GitHub / GitLab app)" (◐/✖/✖/✖).
- **Goal linkage:** pillars 3–4 (agent-native deploy surface) + Render parity — "which of my repos can you deploy?" becomes one MCP call.
- **Expected outcome:** a workspace admin installs the bex GitHub App and every surface can list the granted repos; the connection is stored in the control-plane DB and revocable.
- **Why now:** prerequisite for private-repo deploys and zero-config push-to-deploy (`w2/m9`) and the dashboard create wizard (`w5/m15`) — the last big gap in the deploy story now that in-cluster builds (w1/m5), rootDir (w1/m18), and deploy history (w2/m5) are done. Render parity task included: feature dev across REST/GraphQL/MCP/UI (REST `GET /v1/repos` + MCP repo tools are declared supersets — Render lists repos only via its private dashboard API and its MCP has no repo tools).

## Live DoD evidence (2026-07-12, local CAPD mock cluster)

Run against the real GitHub App **bex-co** (id 2091812, installed org-wide on `bex-co` as installation 90623475), `bex-operator:dev` built from main, `BEX_CP_DB_URI` + `bex-github-app` Secret wired; caller = fresh Kratos identity `m8m9-live@bex.test` (session token).

- **Unconfigured ⇒ 503**: with no `bex-github-app` Secret, authenticated `GET/DELETE /v1/git/connection`, `POST /v1/git/connect`, `GET /v1/repos` all returned `503 {"error":"github integration not configured"}`.
- **Connect**: `POST /v1/git/connect` → `{"connected":false,"installUrl":"https://github.com/apps/bex-co/installations/new"}`; `GET /v1/git/callback?installation_id=90623475` (authenticated — the documented agent path, given the ADR'd browser-callback limitation) → 302, connection recorded (`connected:true, accountLogin:"bex-co"`).
- **Three surfaces, private repo included**: `GET /v1/repos`, GraphQL `repos`, MCP `list_repos` each returned the same 69 repos including private `bex-co/bex-hello-go-live` (`private:true`); MCP `get_git_connection` matched REST/GraphQL connection.
- **Disconnect empties**: `DELETE /v1/git/connection` → 204; `GET /v1/repos` → `[]`; connection `connected:false`. Reconnect via callback → 302, connected again.
- **Dashboard card** (local `yarn dev` against the live bex-api): shows "Connected as bex-co" + install-URL link (screenshot `.playwright-mcp/m8-connect-github-card-connected.png`); Disconnect (type-safe confirm dialog) flips the card to the not-connected state; Connect fires `connectGit` and redirects the browser to `github.com/apps/bex-co/installations/new`. Audit log panel shows the day's `github.StartConnect`/`Connect`/`Disconnect` entries.

## Design decisions (from the brainstorm)

- **GitHub App, not OAuth app or pasted PAT** — short-lived installation tokens (1h), per-repo grants managed on GitHub's side, signed push events for all installed repos. PAT rejected (long-lived stored credential, no per-repo grants, no app webhook).
- **Self-hosters create their own GitHub App** via GitHub's app-manifest flow (one-click creation from a JSON manifest) — this also answers the "who owns the app" question that blocks `w4/003` (GitHub social login, a separate feature).
- **Config via env** (`BEX_GITHUB_APP_ID`, `BEX_GITHUB_APP_PRIVATE_KEY`, `BEX_GITHUB_APP_SLUG`; the webhook secret arrives with m9): any unset ⇒ 503, matching the platform's optional-feature pattern.
- **Storage: control-plane Postgres** (`git_connections`), requires `BEX_CP_DB_URI` — consistent with control-plane opt-in ([docs/ADR003-control-plane.md](../../../docs/ADR003-control-plane.md)).
- **Out of scope:** GitLab/Bitbucket (GitHub-only MVP), PR previews (unblocked by this work, not included), the create wizard (`w5/m15`).
