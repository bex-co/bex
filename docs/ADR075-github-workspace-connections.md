# ADR: GitHub↔workspace connection model — workspace-owned, multi-installation

**Status:** accepted — 2026-08-19, implemented by [w5/m74](../.pm/w5/done/m74/README.md). Amends [ADR026-github-integration.md](ADR026-github-integration.md) §4 (the one-connection-per-workspace data model) and closes its "multi-workspace connection routing is w6" deferral with an explicit decision. Records a deliberate Render-parity divergence for [ADR018-render-parity.md](ADR018-render-parity.md). The live multi-account GitHub install walk (org + personal installation on one workspace) is deferred to a real environment with the GitHub App configured — everything else is covered by unit + integration tests (the full migration chain runs against ephemeral Postgres in CI, `backend-test.yml`).

## Context

### The incident that forced the question

A workspace admin installed the bex GitHub App on their **personal** GitHub account directly from `github.com/apps/<slug>/installations/new` (installation `154851602`), then opened `services/new` and saw no repos. Diagnosis against production:

1. A direct GitHub-side install is a deliberate **no-op** in bex — the `installation` webhook event is answered 200 and discarded (`lego/backend/internal/apps/webhook.go`), because binding requires the three-proof connect flow ([ADR026 §4](ADR026-github-integration.md), w1/m67 F3). Working as designed, but the user had no way to know.
2. The `services/new` "Connect GitHub" prompt links the **bare** install URL from the `gitConnection` query (`dashboard/src/features/services/components/service-source-picker.tsx`) — no signed state, so even a user who dutifully follows the product's own CTA ends in `missing_state`. Only the Settings card's `connectGit` mutation mints a state-bearing URL. A product bug, not a design decision.
3. Even after fixing 1–2 by re-running connect from Settings, the deeper limit remains: `git_connections` is keyed `PRIMARY KEY (workspace_id)` — **one connection per workspace**. The workspace was already bound to the `bex-co` org installation, so connecting the personal account would silently **replace** the org binding rather than add to it. A team that keeps some repos under an org and some under members' personal accounts cannot deploy both from one workspace.

### What Render actually does

Researched from [render.com/docs/github](https://render.com/docs/github), [login-settings](https://render.com/docs/login-settings), [team-members](https://render.com/docs/team-members), and the Dec-2024 [multi-account changelog](https://render.com/changelog/add-your-git-deployment-credentials-to-multiple-render-accounts):

- **The connection is bound to the Render user, not the workspace.** Git deployment credentials live under the individual user's Account Settings. There is no workspace-level git credential object anywhere in Render's model.
- **Installations are never bound to workspaces at all.** They live on the GitHub side (personal account or orgs); Render consumes them through whichever member's connected GitHub identity can see them. One connected GitHub identity can span many installations, so the service-create repo picker enumerates everything the **creating user's** GitHub account reaches — personal + all orgs at once.
- **Per-service credential pointer.** Each service records _which member's_ credential deploys it, swappable in service Settings. Documented failure mode: "if the creator of a Render service loses access to that service's connected GitHub repository, it can disrupt deploys" — remediated by manually re-pointing the service at another member's credential.
- **Direct GitHub-side install is blessed.** Render's docs explicitly send users to `github.com/apps/render/installations/new` to install on a new org or edit repo grants; Render picks the change up through the user's connection.
- Cardinality rules are user-side only: one connected account per provider per Render user; since Dec 2024 one GitHub account may serve **multiple** Render accounts (deploy-only).

### The inconsistency, named

| Dimension | Render | bex today |
| --- | --- | --- |
| Connection owner | user (member) | workspace |
| Installations reachable per workspace | unbounded (union of members' identities) | exactly 1 |
| Installation → workspace binding | none (workspace-agnostic) | strict, unique (w1/m65 F2) |
| Repo picker enumerates | creating user's GitHub identity, all installations | the workspace's single installation |
| Direct github.com install | supported, documented | silent no-op |
| Creator leaves the team | service deploys **break** until credential re-pointed | no effect (workspace owns the connection) |
| Deploy credential | the pointing member's user token | fresh installation token per deploy trigger |

Two of these divergences are bex **strengths** (rows 6–7): workspace ownership means no member-departure breakage and no per-service credential pointer to babysit, and it is what lets the identity-less push webhook, agent-session token mint, and blueprint sync resolve a workspace without any user in the loop. Three are real **gaps** (rows 2, 4, 5): a workspace is capped at one GitHub account, the picker can never show a second account's repos, and the GitHub-side install path every GitHub user already knows is a dead end.

## Decision

### 1. Connections stay workspace-owned — Render's user-bound model is rejected

The connection remains an object of the **workspace**, minted by an admin, visible to and usable by every member per their role. We do not adopt per-user Git deployment credentials or per-service credential pointers.

Rationale, in order of weight:

- **No member-departure breakage.** Render documents the failure and a manual remediation; bex simply doesn't have the failure. A deploy credential that dies when a human leaves is the wrong shape for a team product.
- **Headless consumers need workspace resolution.** The push webhook (no identity), the agent-session token mint (sandbox, not a browser user), blueprint auto-sync, and deploy-time clone-secret refresh all resolve `workspace → installation` with no user in the call path. A user-bound model would force every one of them to pick _some member's_ credential — reintroducing the pointer problem everywhere.
- **Tenancy alignment.** [ADR043](ADR043-tenant-namespace-isolation.md) makes the workspace the isolation unit; repo access is a workspace capability like its namespaces, quotas, and secrets, not a property of whichever member clicked first.
- **Auditability.** One workspace-scoped binding set, admin-gated mutations, is a smaller and more legible trust surface than N members × M providers of personal credentials.

This is a **deliberate, recorded parity divergence**: [ADR018](ADR018-render-parity.md) gets a divergence note on the git-integration rows (the REST/GraphQL/MCP repo surface is a bex extension anyway — Render lists repos only through its private dashboard API).

### 2. A workspace holds **many** installations; an installation still belongs to exactly **one** workspace

The cardinality changes from 1:1 to **N:1** (N installations per workspace, each installation in at most one workspace):

- **`git_connections` is re-keyed:** `PRIMARY KEY (installation_id)` (promoting the existing `git_connections_installation_idx` unique index), with a non-unique index on `workspace_id`. Columns are unchanged (`workspace_id`, `installation_id`, `account_login`, `created_at`). The migration is metadata-only for existing data — the current production table holds one row per workspace by construction, so no row rewrite, dedup, or backfill is needed.
- **One-workspace-per-installation is kept** (w1/m65 F2). Sharing an installation across workspaces is rejected: `WorkspaceForInstallation` must stay a function (the push webhook fans a signed delivery into exactly one tenant scope, ADR057 round-6 #9 fail-closed), and cross-workspace sharing would let one workspace's admin grant another workspace's repos by side effect. A workspace that wants an already-bound installation gets the same `ErrConflict` as today.
- Within a workspace, `account_login` is unique by construction (GitHub allows one installation of a given App per account), so **repo owner → connection** resolution inside a workspace is unambiguous. `account_login` is refreshed from the app-JWT installation lookup whenever the row is touched, so a GitHub account rename converges instead of sticking.
- **Quota:** `BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE` (default `10`, `0` disables) bounds one tenant's connection fan-out — the same abuse-bound pattern as `BEX_MAX_ENV_GROUPS_PER_WORKSPACE`. Exceeding it refuses connect with coded `GIT_CONNECTION_LIMIT` (409) identically across REST/GraphQL/MCP. The cap also bounds the repo-list fan-out (§4).

### 3. The connect flow is unchanged; every entry point must use it

The three-proof, one-principal connect flow ([ADR026 §4](ADR026-github-integration.md): signed nonce-only state + server-side single-use transaction + initiator-subject match + OAuth installation-admin proof + fresh `can_manage` at callback) is untouched — it is the security core of this feature and it already supports "the app is installed, bind it" (GitHub preserves `state` through the configure flow as well as first install; the app's **"Redirect on update"** setting must be enabled so grant edits also round-trip the Setup URL).

What changes is that **every Connect CTA goes through it**:

- The `services/new` source picker's "Connect GitHub" button switches from the `gitConnection` query's bare `installUrl` to the `connectGit` **mutation** (StartConnect → stateful URL), exactly like the Settings card. The bare install URL **stops being advertised**: `GetConnection`/`gitConnection` drop `installUrl` from the not-connected view (the field stays for connected rows as a "configure repo grants on GitHub" deep link, where statelessness is fine because configuration edits don't bind anything).
- Direct GitHub-side installs remain a **no-op for binding** (auto-bind from the `installation` webhook is rejected again — the webhook proves the installation exists, not that any bex principal may attach it to any workspace). But the dead end gets signage: the callback's `missing_state` failure redirect now lands on a dashboard page that explains "this installation isn't connected to a workspace yet" and offers the stateful connect button for the caller's admin workspaces — turning the incident's silent failure into a one-click recovery.

### 4. Consumption resolves by repo owner within the workspace's connection set

Every consumer that today does `GetGitConnection(workspace)` moves to owner-scoped resolution against the workspace's set:

- **`ListRepos`** aggregates across all of the workspace's connections — one fresh installation token per connection, fetched concurrently, each repo annotated with its `accountLogin`/`installationId` so the picker can group by account (Render-parity UX: personal + org repos in one list). Fan-out is bounded by the §2 quota.
- **`ListBranches`, deploy-time clone-token mint, blueprint fetch, commit resolution:** parse `owner` from the repo URL via the existing structural `githubOwnerRepo` gate (unchanged — w1/m65 F1), then select the workspace connection whose `account_login == owner`; no match ⇒ exactly today's no-connection behavior (public fetch or clear failure, never a wrong-installation token). `RepoAccessible` stays as the post-mint grant check.
- **Push webhook:** unchanged — `WorkspaceForInstallation` still resolves the delivery's installation to its one workspace.
- **Agent-session token mint:** the owner-equals-`account_login` guard becomes owner-in-connection-set (still exact-match, still one repository, still `contents:write` least privilege).

### 5. Surfaces

- **REST:** `GET /v1/git/connections` (list), `DELETE /v1/git/connections/{installationId}` (admin-only, per-connection disconnect). The singular `GET`/`DELETE /v1/git/connection` remain as compatibility aliases over the first/only connection and are documented deprecated (this whole surface is a bex extension, so the deprecation is ours to schedule).
- **GraphQL:** `gitConnections: [GitConnection!]!` alongside the existing singular (deprecated directive); `disconnectGit(installationId)` gains the argument (omitted ⇒ sole connection, error when ambiguous).
- **MCP:** `get_git_connection` → `list_git_connections`; `list_repos` returns the aggregated, account-annotated list.
- **Dashboard:** the Settings card lists all connections with per-row disconnect + "Connect another account"; the `services/new` picker groups repos by account and uses the stateful connect (§3).

## Alternatives considered

- **Adopt Render's user-bound credentials wholesale** — rejected. It imports Render's documented member-departure failure mode and per-service credential pointer, and it breaks every headless consumer (webhook routing, agent mint, blueprint sync) that must resolve a workspace without a user present. Render's shape is a consequence of Render's history (login OAuth first, App later), not a design to copy.
- **Hybrid: workspace connections + per-user connections merged in the picker** — rejected for now. It doubles the credential model (two ownership classes, two revocation stories, precedence rules) to serve one marginal case — a member who wants to deploy from repos the workspace admins have not connected — which is arguably a feature, not a gap: workspace admins _should_ control the deployable repo universe.
- **Share one installation across workspaces (drop one-workspace-per-installation)** — rejected. Webhook scoping stops being a function, w1/m65 F2's cross-tenant attach protection erodes, and the legitimate need ("my personal account serves my two workspaces") is better served by GitHub's own model — install the App per-org, or accept the 1-workspace bind for a personal account.
- **Auto-bind on the `installation` webhook** — rejected again, as in ADR026: the webhook proves existence, not authority. Binding without the three proofs reopens the exact cross-tenant installation-attach the transaction flow closed (w1/m67 F3).
- **Keep 1:1 and just fix the dashboard CTAs** — rejected as insufficient: it fixes the trap but leaves the incident's real want (org **and** personal repos in one workspace) impossible, and leaves us inconsistent with Render on the one dimension where Render's UX is strictly better.

## Consequences

- The incident's desired end state becomes real: one workspace connected to both the `bex-co` org installation and the `puncsky` personal installation, both accounts' repos in one picker, each deploy minting a token from the right installation.
- The migration is a key swap on a table whose production contents are one row; no data movement, no dual-read window, no runbook — deliberately unlike [ADR074](ADR074-workspace-scoped-artifact-identity.md)'s identity migration.
- Every security invariant of ADR026 and the review lineage is preserved by construction: the three-proof connect, fresh `can_manage` at callback (ADR073 #3), unique installation→workspace (w1/m65 F2), fail-closed multitenant webhook scoping (ADR057), structural repo-origin gating before any mint (w1/m65 F1), token least-privilege and narrowing (ADR047/ADR056 F14 unchanged). New attack surface is limited to the connection **count**, which the quota bounds.
- `ListRepos` latency grows with connection count (one GitHub round trip per connection, concurrent); acceptable under the default cap of 10 and pageable per account if it ever isn't.
- The bare install URL disappearing from the disconnected view is a small API-shape change to a bex-extension surface; the dashboard is its only known consumer.
- Render-parity ledger and [docs/ADR018](ADR018-render-parity.md) gain the recorded divergence: bex is **workspace-bound by design** (superset UX: multi-account picker; subset: no per-member credentials).

## Verification

1. **Multi-connection connect** — with an org installation already bound: Settings → Connect another account → GitHub account picker → choose a personal account → callback binds a **second** row; `GET /v1/git/connections` lists both; `services/new` shows both accounts' repos grouped; creating a service from each account clones with that account's installation token (assert distinct `installation_id` in the mint audit).
2. **Uniqueness + quota** — connecting an installation already bound to another workspace still returns 409/`ErrConflict`; the N+1th connection over `BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE` returns `GIT_CONNECTION_LIMIT` on REST, GraphQL, and MCP identically.
3. **Entry-point consistency** — the `services/new` connect button round-trips GitHub and binds successfully (no `missing_state`); a direct `github.com/apps/<slug>/installations/new` install lands on the recovery page, and completing its stateful connect binds the already-present installation without re-installing.
4. **Owner-scoped consumption** — with two connections, a push webhook from each installation redeploys only its own workspace's matching Apps; agent-session mint refuses a repo whose owner matches **no** workspace connection; deploy of a repo from account B never receives account A's token (assert in the clone-secret refresh path).
5. **Compatibility** — a workspace with exactly one connection sees byte-identical behavior on the singular REST/GraphQL surfaces; the migration up/down round-trips the current production row unchanged.
