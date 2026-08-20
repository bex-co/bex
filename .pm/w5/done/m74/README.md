# w5 · m74 — GitHub↔workspace connections: multi-installation model (ADR078)

**Worker:** worker5 **Goal:** a workspace can hold N GitHub App installations (org + personal in one repo picker), every Connect CTA goes through the stateful three-proof flow, and a direct github.com install gets a recovery path instead of a silent dead end **Status:** done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Store + migration: re-key `git_connections` to N-per-workspace + connection quota          | 45m | — — **DONE**          |
| t002 | Connect flow: additive bind under quota, `account_login` refresh, conflict gate unchanged  | 30m | t001 — **DONE**       |
| t003 | Owner-scoped consumption: aggregate ListRepos; owner→connection resolution at every mint   | 60m | t002 — **DONE**       |
| t004 | Surfaces: connections list + per-installation disconnect ×3 (REST/GraphQL/MCP), aliases    | 45m | t003 — **DONE**       |
| t005 | Dashboard: multi-connection Settings card, stateful services/new connect, recovery page    | 60m | t004 — **DONE**       |
| t006 | Render parity — cross-surface consistency + record the deliberate divergence in ADR018     | 30m | t005 — **DONE**       |
| t007 | Simplify — `/simplify` over the milestone's changed code                                   | 30m | t006 — **DONE**       |
| t008 | Test coverage — migration, quota, owner resolution, entry points, compatibility            | 45m | t006 — **DONE**       |
| t009 | Closeout — verify DoD, sync status, move to done/                                          | 15m | t008 — **DONE**       |

**Done 2026-08-19.** All code, tests, and docs landed and verified: full migration chain (0001→0091) up/down + down-guard against real Postgres, multi-connection store + service + surface tests green, dashboard typecheck/lint/tests green, `make lint-backend` clean, `/simplify` applied (errgroup→WaitGroup, deduped dashboard query hook). The one deferred DoD element — a live browser + GitHub App install of a second account onto one workspace — is recorded as open note `w5/046` (needs a real environment with `BEX_GITHUB_APP_*`).

## Definition of done

[docs/ADR078-github-workspace-connections.md](../../../docs/ADR078-github-workspace-connections.md) is implemented and its five verification points hold:

1. A workspace already bound to an org installation can connect a second (personal) installation from Settings; `GET /v1/git/connections` lists both; `services/new` shows both accounts' repos grouped by account; a deploy from each account clones with that account's own installation token.
2. Connecting an installation bound to another workspace still 409s (`ErrConflict`); exceeding `BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE` refuses with coded `GIT_CONNECTION_LIMIT` identically on REST, GraphQL, and MCP.
3. The `services/new` connect button round-trips GitHub and **binds** (no `missing_state`); a direct `github.com/apps/<slug>/installations/new` install lands on a dashboard recovery page whose stateful connect binds the already-present installation.
4. With two connections, each installation's push webhook redeploys only its own workspace's matching Apps; agent-session mint refuses a repo owner matching no workspace connection; account B's repo never receives account A's token.
5. A single-connection workspace sees byte-identical behavior on the singular REST/GraphQL surfaces; the migration round-trips the current production row unchanged.

## Source + Goal linkage

- **Source:** [docs/ADR078-github-workspace-connections.md](../../../docs/ADR078-github-workspace-connections.md) (proposed 2026-08-19), handed off via `/pm` the same day. Root incident: a workspace admin installed the App on their personal account directly from github.com (installation 154851602) and `services/new` showed no repos — direct install is a designed no-op, the `services/new` CTA links the stateless bare install URL (guaranteed `missing_state`), and the `PRIMARY KEY (workspace_id)` schema caps a workspace at one installation so org + personal repos can never coexist.
- **Goal linkage:** Render parity (pillar: the open-source Render alternative, [docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)) on the one git-integration dimension where Render's UX is strictly better (multi-account repo picker), while keeping bex's deliberate workspace-owned divergence (no member-departure deploy breakage; headless webhook/agent/blueprint consumers keep workspace resolution). Directly serves pillars 3–4 ("which of my repos can you deploy?" / push-to-deploy).
- **Expected outcome:** one workspace connected to both the `bex-co` org and a personal installation, both accounts' repos in one picker, correct per-installation token minting — and no user can ever again follow a product CTA into the `missing_state` dead end.
- **Why now:** a real user hit all three gaps in production on 2026-08-19; the two dashboard traps actively mislead anyone connecting GitHub today, and the schema relax is a metadata-only migration while production `git_connections` still holds exactly one row (no dual-read window needed — deliberately unlike ADR074's identity migration).
- **Render parity task included:** yes — the milestone changes REST/GraphQL/MCP surfaces and the dashboard; ADR018's git rows must record the divergence + superset.

## Security invariants (must hold throughout — from ADR078/ADR026)

Three-proof one-principal connect (w1/m67 F3) untouched; fresh `can_manage` at callback (ADR073 #3); one-workspace-per-installation kept (w1/m65 F2); fail-closed multitenant webhook scoping (ADR057 round-6 #9); structural `githubOwnerRepo` origin gate before any mint (w1/m65 F1); token least-privilege/narrowing (ADR047, ADR056 F14). No auto-bind from the `installation` webhook.
