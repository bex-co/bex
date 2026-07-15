# ADR: GitHub integration — a GitHub App for private-repo deploys and zero-config push-to-deploy

**Status:** accepted — 2026-07-11. Implements the connect+list half in [w2/m8](../.pm/w2/m8/README.md) (this milestone) and the private-clone + hands-free push-to-deploy half in [w2/m9](../.pm/w2/m9/README.md). Optional feature: unset config ⇒ 503 on every git-connect verb, exactly as [ADR013-secrets.md](ADR013-secrets.md)/[ADR003-control-plane.md](ADR003-control-plane.md) gate their features.

## Context

[docs/ADR008-vision.md](ADR008-vision.md) pillars 3–4 want "which of my repos can you deploy?" to be one agent call, and "a later git push redeploys" to be true for **private** repos with **no per-repo setup**. Two things were missing:

1. A **managed connection** to a git host, so bex can enumerate a workspace's repos and clone private ones — today [ADR017-deploy-from-chat.md](ADR017-deploy-from-chat.md) can only build a public `spec.repo` and only redeploys via a **manually configured** per-repo webhook holding the shared `BEX_WEBHOOK_SECRET`.
2. A credential the in-cluster build ([w1/m5](../.pm/w1/m5/README.md)) can use to clone a **private** repo, minted without teaching the operator anything about GitHub.

The parity target is Render's model ([render.com/docs/github](https://render.com/docs/github)): install the Render GitHub App with an all-repos or selected-repos grant → the granted repos appear in the create flow → private clones just work → the app delivers **signed push events** for every installed repo, so auto-deploy needs no per-repo webhook.

The rule this works within (from [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md), [ADR003-control-plane.md](ADR003-control-plane.md)): product concerns live in **bex-api**; the operator is DB-free mechanism. bex-api owns GitHub; the operator only consumes an opaque k8s Secret.

## Decision

### 1. A GitHub App — not an OAuth app, not a pasted PAT

bex integrates GitHub through a **GitHub App**. The app gives us exactly the three primitives the parity target needs:

- **Short-lived installation tokens** (1h) minted on demand from the app's private key — never a long-lived stored credential.
- **Per-repo grants** managed on GitHub's side (the install screen), so the workspace admin, not bex, decides which repos are exposed.
- **One app-wide webhook** configured once at app-creation time that delivers **signed** push events for every installed repo — the foundation of zero-config push-to-deploy.

A **personal access token** was rejected: it is a long-lived secret bex would have to store and rotate, it grants all of a user's repos with no per-repo scoping, and it carries no app-level webhook (push-to-deploy would stay manual). A plain **OAuth app** was rejected too: OAuth apps authorize _as a user_ (again broad, user-scoped, revoked when the user leaves) and have no installation model or per-installation webhook — they answer "log in with GitHub," not "grant these repos to this workspace." (GitHub social **login** is a separate feature, [w4/003](../.pm/w4/003.md); it can reuse the same app's client id — see §6.)

### 2. Self-hosters mint their own app via the manifest flow

bex is open-source and self-hosted, so there is no single "bex GitHub App" — each operator creates their own. GitHub's **app-manifest flow** makes this one click: the operator opens a bex-provided form that POSTs a JSON manifest (name, permissions `contents:read` + `metadata:read`, the `push` webhook event, the callback/webhook/setup URLs) to `github.com/settings/apps/new?state=…`; GitHub creates the app and redirects back with a temporary code; a one-time `POST /app-manifests/{code}/conversions` exchanges it for the app id, slug, private key, and webhook secret, which the operator drops into their `.env`. This is the same "who owns the app for self-hosters" question that blocks GitHub social login ([w4/003](../.pm/w4/003.md)) — the manifest answer unblocks it.

Manifest generation is a small helper; wiring the created values into config is manual (secrets stay out-of-band, [ADR012-auth.md](ADR012-auth.md)), so the manifest **conversion endpoint is not automated by bex** — the operator pastes the four values. This keeps the private key on the same out-of-band path as every other platform secret.

### 3. Config is env-var-based; unset ⇒ 503

Three variables configure the app, all read once at startup like every other optional feature:

| Variable | Meaning |
| --- | --- |
| `BEX_GITHUB_APP_ID` | numeric app id — the `iss` of the app JWT |
| `BEX_GITHUB_APP_PRIVATE_KEY` | the app's RSA private key (PEM), an **out-of-band secret** ([ADR012-auth.md](ADR012-auth.md)) — signs the app JWT (RS256); its PEM bytes also HMAC-sign the short-lived browser callback state |
| `BEX_GITHUB_APP_SLUG` | the app's slug — builds the install URL `github.com/apps/<slug>/installations/new` |

Any of the three unset ⇒ the github service is nil ⇒ **every** git-connect verb (connect, callback, get/delete connection, list repos) returns **503**, matching `BEX_OPENBAO_URL`/`BEX_CP_DB_URI`. The push webhook's second key, **`BEX_GITHUB_WEBHOOK_SECRET`**, is introduced by [w2/m9](../.pm/w2/m9/README.md) (the app's webhook HMAC key); it is independent of these three and gates only the webhook's GitHub-signed path.

### 4. Connections live in the control-plane Postgres

A connection (which GitHub installation belongs to this workspace) is durable state, so it lives in the control-plane store (`git_connections`, keyed by workspace), which requires `BEX_CP_DB_URI` — consistent with [ADR003-control-plane.md](ADR003-control-plane.md)'s opt-in. One connection per workspace for now (bex is effectively single-workspace; multi-workspace is w6). Recording a connection **validates** it first by fetching the installation from GitHub with the app JWT (`GET /app/installations/{id}`, which also yields the account login) — a forged `installation_id` bex can't authenticate against returns 404 and is rejected before anything is persisted, which is what lets the browser-redirect callback (§5) trust an unauthenticated query parameter.

### 5. Surfaces — REST/GraphQL/MCP/UI, with the repo surface a bex superset

The connection and repo list are exposed on all four surfaces over one core (the [ADR006-bex-api.md](ADR006-bex-api.md) one-core/thin-adapters rule):

- **REST:** `POST /v1/git/connect` → `{installUrl}` (the URL includes a 15-minute HMAC-signed `state` credential binding the already-authorized workspace); `GET /v1/git/callback?installation_id=…&state=…` (GitHub's post-install "Setup URL" target — verifies state, records the installation against that workspace, then redirects to the dashboard via `BEX_DASHBOARD_URL`, set to `https://dashboard.bex.co` in the production API manifest); `GET`/`DELETE /v1/git/connection`; `GET /v1/repos` (paginated repo list).
- **GraphQL:** `gitConnection` + `repos` queries; `connectGit`/`disconnectGit` mutations.
- **MCP:** `list_repos` and `get_git_connection` tools — the agent-facing payoff ("which repos can I deploy?" + "how does the human connect?").
- **UI:** a Settings → "Connect GitHub" card (install link, connected account, disconnect).

`GET /v1/repos` and the MCP repo tools are **bex extensions** (supersets): Render lists repos only through its private dashboard API and its MCP has no repo tools, so the comparison target is bex's own cross-surface consistency, not a Render shape — flagged in-code and recorded in [ADR018-render-parity.md](ADR018-render-parity.md) § "bex ahead of Render". The callback authenticates differently from the rest: GitHub redirects a browser to it with no bearer or dashboard cookie, so `GET /v1/git/callback` is the one exact method+path exception to the shared HTTP auth gate. Its credential is the HMAC-signed, expiring [`state` GitHub preserves through an App install](https://docs.github.com/en/apps/sharing-github-apps/sharing-your-github-app), minted only after `StartConnect` authorizes `can_manage`; the callback verifies the signature and expiry before trusting the encoded workspace. It then independently fetches `installation_id` from GitHub (§4) before persisting. Missing, tampered, or expired state is rejected and redirects to `/settings?git_error=<bounded-code>` for visible dashboard feedback; callback redirects set `Referrer-Policy: no-referrer` so the credential-bearing API URL does not leak to the dashboard. An already-authenticated API/agent caller can still omit state and use the original `Connect` authorization path, preserving that workflow.

### 6. Private-repo clone — a token in a Secret, the operator stays GitHub-free (w2/m9)

The build mechanism must clone private repos without knowing about GitHub. So:

- **Operator:** a new optional `App.spec.cloneSecret` names a k8s Secret (in the App's namespace, key `token`). When set, the BuildKit build Job mounts it and passes it as BuildKit's standard **`GIT_AUTH_TOKEN`** build secret, which authenticates the https git-context fetch (`x-access-token` basic auth). Unset ⇒ today's public clone, byte-identical. The operator never mints or refreshes tokens — an absent/expired Secret just fails the build with a clear condition. When the build Job runs in a separate `BEX_BUILD_NAMESPACE` (e.g. `bex-system` in prod, not the App's namespace), the operator **relocates** the clone Secret into the build namespace before starting the build (a mechanism-only opaque-byte copy) — otherwise the build pod is `CreateContainerConfigError` ("secret not found"). Verified live on prod 2026-07-12.
- **bex-api:** on every deploy-triggering verb (`Create`/upsert, MCP `deploy`, and the webhook redeploy) whose `spec.repo` belongs to the workspace's connection, bex-api mints a **fresh** installation token, writes/refreshes the `<app>-clone` Secret, and sets `spec.cloneSecret` — so each build starts with a token minted seconds ago. Public/unconnected repos are untouched. The clone Secret is labeled managed-by bex-api and removed by the service-delete cascade. **Workspace resolution:** the connection is keyed by workspace, so the deploy path resolves the workspace from the **App CR's `bex.co/tenant` label** (falling back to the caller's tenant, then the default workspace) — this lets the no-identity push webhook still find a tenant-owned connection to refresh the token, rather than defaulting to the wrong workspace.

### 7. Zero-config push-to-deploy — the app webhook, a second accepted key (w2/m9)

The GitHub App's one app-wide webhook delivers push events (signed with `BEX_GITHUB_WEBHOOK_SECRET`) for **every** installed repo to the existing `POST /v1/webhooks/git`. The handler verifies `X-Hub-Signature-256` against **both** `BEX_WEBHOOK_SECRET` (the existing manual key) **and** `BEX_GITHUB_WEBHOOK_SECRET` in constant time — valid under either ⇒ accept; **503 only when neither is set**. GitHub's own lifecycle deliveries (`ping`, `installation`) get a 200 no-op when the signature is valid (never a 401 on GitHub's health checks). Everything downstream of signature verification — repo canonicalization across URL forms, branch/`rootDir` match, `autoDeploy` gate — is reused unchanged from [ADR017-deploy-from-chat.md](ADR017-deploy-from-chat.md). The result: a `git push` to a tracked branch of an installed repo redeploys, with **no per-repo webhook configuration**.

## Alternatives considered

- **Personal access token** — rejected: long-lived stored secret, no per-repo grant, no app-level webhook (push-to-deploy stays manual).
- **Plain OAuth app** — rejected: authorizes as a user (broad, user-scoped, dies when the user leaves), no installation model, no per-installation webhook. It answers "log in with GitHub," a different feature ([w4/003](../.pm/w4/003.md)).
- **A single hosted bex GitHub App** — impossible for an open-source, self-hosted product: each operator owns their install, their private key, their webhook. The manifest flow makes per-operator app creation one click.
- **Storing the connection in a k8s CR / OpenBao instead of Postgres** — rejected: a connection is workspace-scoped relational state that the control-plane store already owns; OpenBao is for tenant credentials, not platform metadata.
- **Teaching the operator to mint tokens** — rejected: violates the DB-free, mechanism-only operator boundary. bex-api mints; the operator consumes an opaque Secret.
- **A bespoke `POST /v1/deploy`-style clone endpoint** — unnecessary: the token/Secret write rides the existing `Create`/`redeploy` verbs ([ADR017-deploy-from-chat.md](ADR017-deploy-from-chat.md)), no new verb.

## Consequences

- **Private repos become first-class deploy sources.** Connect once → the granted repos list on every surface → `deploy` clones them with a fresh token.
- **Push-to-deploy needs no per-repo setup.** The app's app-wide signed webhook covers every installed repo; the manual `BEX_WEBHOOK_SECRET` path stays supported (a repo not in any connection, or a self-hoster who hasn't made an app).
- **Accepted limitation — the 1h token and slow operator retries.** Installation tokens live 1h. bex-api mints a fresh one at every deploy trigger, so a normal build (minutes) is fine. But an **operator-side build retry more than 1h after its trigger** finds an expired clone token and fails until the next deploy re-mints. This is accepted (not worked around) because refreshing tokens for arbitrarily-delayed retries would require the operator to hold GitHub credentials, breaking the mechanism-only boundary. The failure is a clear build condition, not a silent public-clone fallback.
- **Trust boundary.** The app private key (in `.env`, out-of-band) can mint tokens for every installation of the app and HMAC-sign 15-minute workspace callback state; the webhook secret can trigger redeploys of matching installed repos — the same trust shape as the other platform secrets. Installation tokens are scoped (`contents:read`, `metadata:read`) and expire in 1h.
- **Single workspace for now.** One connection per workspace; multi-workspace connection routing is w6.
- **Browser callback credential (w2/m34).** The dashboard begins through authenticated `connectGit`, which returns an install URL carrying the signed workspace state described in §5. GitHub passes that value back unchanged; bex-api verifies it without requiring the dashboard's host-scoped Kratos cookie, records the validated installation, and returns the user to `/settings`. The original Bearer/session callback path remains supported for API and agent callers.

## Verification

1. **Connect + list** — with the three `BEX_GITHUB_APP_*` vars + `BEX_CP_DB_URI` set: `POST /v1/git/connect` → install URL with signed state; install on a real account (private repo granted) in an ordinary dashboard browser with no bex-api cookie → GitHub's Setup-URL callback verifies state and records the installation in the workspace that initiated it; the browser lands on `/settings` showing the connection; missing/tampered/expired state lands there with a visible error. The authenticated Bearer callback still records normally. `GET /v1/repos`, GraphQL `repos`, MCP `list_repos` all return that installation's repos **including the private one**; `DELETE /v1/git/connection` empties them; any `BEX_GITHUB_APP_*` unset ⇒ every git-connect verb 503s. (w2/m8 + w2/m34 DoD.)
2. **Private deploy + hands-free push** — create a service from the private repo → the in-cluster build clones with the installation token (no auth error) → live URL; `git push` to the tracked branch → the app's signed delivery redeploys (new revision in deploy history) with **zero manual webhook config**; `autoDeploy: false` suppresses the next push; a tampered signature → 401. (w2/m9 DoD.)
