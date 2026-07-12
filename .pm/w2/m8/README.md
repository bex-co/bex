# w2 · m8 — Connect GitHub: GitHub App connection + repo listing

**Worker:** worker2 **Goal:** an admin connects a GitHub account once (bex GitHub App install); agents and the dashboard can enumerate its repos on all four surfaces. **Status:** code-complete (t001–t009 built + unit-tested + lint-clean); t010 live acceptance pending a real GitHub App + cluster run (see t010).

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | ADR `docs/github-integration.md` — GitHub App model, manifest flow, env vars, trust chain | 40m | —          | — **DONE** |
| t002 | `lego/backend/internal/github` client — app JWT, installation tokens, repo listing        | 45m | t001       | — **DONE** |
| t003 | Control-plane store: `git_connections` table + connection verbs (authz-gated)             | 40m | t002       | — **DONE** |
| t004 | REST: connect · callback · get/delete connection · `GET /v1/repos`                        | 45m | t003       | — **DONE** |
| t005 | GraphQL (`gitConnection`/`repos` + mutations) and MCP (`list_repos`, `get_git_connection`) | 40m | t004       | — **DONE** |
| t006 | Dashboard: Settings → "Connect GitHub" card (install link, status, disconnect)            | 45m | t005       | — **DONE** (code; UI needs `yarn codegen` + build to verify) |
| t007 | Render parity — cross-surface consistency + declare REST/MCP repo surface a bex superset  | 30m | t006       | — **DONE** |
| t008 | Simplify — `/simplify` over the milestone's diff                                          | 30m | t007       | — **DONE** (run jointly with m9/t007 over the combined diff) |
| t009 | Test coverage — connection lifecycle, 503-when-unconfigured, token minting, pagination    | 40m | t007       | — **DONE** |
| t010 | Closeout — DoD verified, move to `done/`                                                  | 15m | t009       | — **OPEN** (needs a real GitHub App + cluster live run; runbook in t010) |

## Definition of done

With `BEX_GITHUB_APP_ID`/`BEX_GITHUB_APP_PRIVATE_KEY`/`BEX_GITHUB_APP_SLUG` + `BEX_CP_DB_URI` set, connecting a real GitHub account (install → callback) makes `GET /v1/repos`, GraphQL `repos`, and MCP `list_repos` return that installation's repos (private included); disconnect empties them; any GitHub env var unset ⇒ every git-connect verb returns 503; the dashboard Settings card shows, creates, and removes the connection. Verified live against a real GitHub account.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-07-11 (GitHub-connect topic, Render docs `render.com/docs/github` + `/web/new` flow); closes the connect+list half of [docs/render-parity.md](../../../docs/render-parity.md) row "Git connections (GitHub / GitLab app)" (◐/✖/✖/✖).
- **Goal linkage:** pillars 3–4 (agent-native deploy surface) + Render parity — "which of my repos can you deploy?" becomes one MCP call.
- **Expected outcome:** a workspace admin installs the bex GitHub App and every surface can list the granted repos; the connection is stored in the control-plane DB and revocable.
- **Why now:** prerequisite for private-repo deploys and zero-config push-to-deploy (`w2/m9`) and the dashboard create wizard (`w5/m15`) — the last big gap in the deploy story now that in-cluster builds (w1/m5), rootDir (w1/m18), and deploy history (w2/m5) are done. Render parity task included: feature dev across REST/GraphQL/MCP/UI (REST `GET /v1/repos` + MCP repo tools are declared supersets — Render lists repos only via its private dashboard API and its MCP has no repo tools).

## Design decisions (from the brainstorm)

- **GitHub App, not OAuth app or pasted PAT** — short-lived installation tokens (1h), per-repo grants managed on GitHub's side, signed push events for all installed repos. PAT rejected (long-lived stored credential, no per-repo grants, no app webhook).
- **Self-hosters create their own GitHub App** via GitHub's app-manifest flow (one-click creation from a JSON manifest) — this also answers the "who owns the app" question that blocks `w4/003` (GitHub social login, a separate feature).
- **Config via env** (`BEX_GITHUB_APP_ID`, `BEX_GITHUB_APP_PRIVATE_KEY`, `BEX_GITHUB_APP_SLUG`; the webhook secret arrives with m9): any unset ⇒ 503, matching the platform's optional-feature pattern.
- **Storage: control-plane Postgres** (`git_connections`), requires `BEX_CP_DB_URI` — consistent with control-plane opt-in ([docs/control-plane.md](../../../docs/control-plane.md)).
- **Out of scope:** GitLab/Bitbucket (GitHub-only MVP), PR previews (unblocked by this work, not included), the create wizard (`w5/m15`).
