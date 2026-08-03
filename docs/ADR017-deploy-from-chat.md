# ADR: deploy-from-chat — create-as-deploy + HMAC push-to-deploy

**Status:** accepted — implemented 2026-07-08 in bex-api (`lego/backend/internal/apps`: `Create`/`Deploy`/webhook, over REST/GraphQL/MCP), unit- and integration-tested. The **live** "repo → https URL" leg needs in-cluster builds ([w1/m5](../.pm/w1/m5/README.md)); until that lands the loop is proven at the handler level, not end-to-end on a cluster.

## Context

[docs/ADR008-vision.md](ADR008-vision.md) pillar 4 is "deploy-from-chat": one API call takes a repo + its `render.yaml` to a live https URL, so "deploy this" is a single agent action, and a later git push redeploys. bex already has the mechanism — the operator builds `App.spec.repo` from git and serves it; [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md) established that a product action is **a word in the `App` CR contract**, not an operator endpoint. What was missing was the _product verb_ on bex-api and the _push trigger_.

Two shapes were on the table for the verb. The original `.pm/w2/m2/t001` proposed a bespoke `POST /v1/deploy`. But [w2/m4](ADR006-bex-api.md) was already adding Render's own create surface (`POST /v1/services` / `create_web_service` over one `Core.Create`) — and a Render web service _is_ a repo + config, which is exactly what "deploy" carries. A second endpoint that also turns a repo into a running service would be a parallel surface that drifts from the create one. So t001 was **amended 2026-07-08** to ride `Core.Create` instead. This ADR records the resulting decisions, because they set the precedent for the next verbs (rollback, deploy history — [w2/m5](../.pm/w2/m5/README.md)).

The rule this works within (from [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md), [ADR003-control-plane.md](ADR003-control-plane.md)): product action → **App CR spec** (intent) → operator converges. bex-api writes intent and never calls the operator; the operator keys a rebuild on `metadata.generation` changing (a genuine spec bump, not suspended, repo-backed → re-run build-from-git).

## Decision

### 1. Deploy _is_ create — no bespoke endpoint

Deploy-from-chat rides the same `Core.Create` every other client uses. Two entry points, one verb:

- **`deploy {repo, branch?, bexYaml}`** (MCP) — the agent-facing verb: the wire argument name is retained for compatibility, but its content is the strict `render.yaml` contract. The `repo`/`branch` args override the manifest (an agent that already knows the checkout needn't duplicate it).
- **`create_web_service` / `POST /v1/services` / `createService`** — the structured create surface (a `repo` or a prebuilt `image`) across MCP/REST/GraphQL.

The `render.yaml` → spec field mapping is the same compiler path [`scripts/app-apply.sh`](../scripts/app-apply.sh) calls through the Blueprint API, so a manifest that validates with the script deploys through the API. There is no `POST /v1/deploy`.

### 2. Create is an upsert; a repeat deploy is a redeploy

`Create` is **create-or-update**: a new name creates the `App` CR; an existing name re-applies the request's spec fields and bumps `spec.restartedAt`. So calling `deploy` twice for the same service redeploys it — never a duplicate — which is exactly "push the button again" from a chat. The update path re-applies only create-owned fields and leaves operator/other-feature fields alone (`EnvFromSecret` from the [secrets](ADR013-secrets.md) env-vars API, `Suspended`, `IdleTTLSeconds`, `AutoDeploy`) — the same discipline the control-plane projector's `applyOwnedSpec` follows.

**Update (w4/m19):** the "one verb" premise above forked. Render's own duplicate-name behavior ([docs/render-artifacts/duplicate-service-names.md](render-artifacts/duplicate-service-names.md)) turned out to require the opposite of upsert on the **interactive** create surface: a same-workspace duplicate `name` on `create_web_service`/`POST /v1/services`/`createService` is now a hard **409 reject** ("name already in use"), never a silent redeploy — see [ADR018-render-parity.md](ADR018-render-parity.md)'s Create service row. `deploy` and the Blueprint stack path (`render.yaml` apply, `scripts/app-apply.sh`) keep the upsert behavior described in this section unchanged — they now go through a dedicated idempotent-upsert function (`applyCreate`, `lego/backend/internal/apps/deploy.go`) forked off the interactive `create` at the same commit, rather than sharing `Create` directly. So `deploy` still redeploys on a repeat call for the same name; `create_web_service` no longer does — a caller wanting "redeploy this" over the structured create surface uses `restart_service`, not a second `create_web_service` call.

### 3. Redeploy reuses `spec.restartedAt` — no new contract word

Redeploy needs the operator to roll a fresh revision. [ADR007-restart-suspend-and-resume.md](ADR007-restart-suspend-and-resume.md) added `spec.restartedAt` as the verb-as-timestamp for that intent; it changes the explicit artifact/release fingerprints even though ordinary operational generations do not. For a **repo-backed** App this re-runs build-from-git; for an **image-backed** App it is a rolling restart of the same image. So redeploy = stamp `spec.restartedAt` — no `spec.deployID`/revision field invented, the same field the upsert and the webhook both write.

### 4. Create writes the App CR directly (the hand-applied path)

The public create surface has no tenant context, so it writes the `App` CR **directly** (like `scripts/app-apply.sh`), not through a control-plane `apps` row. The row-backed, multi-tenant create — which needs a `tenantId` and mints a `<tenant>-<app>` CR name — stays the internal control-plane API's job ([ADR003-control-plane.md](ADR003-control-plane.md), `store` `POST /v1/apps`). The two coexist safely: the projector lists and deletes **only** CRs carrying its `app.kubernetes.io/managed-by: bex-controlplane` label, so it never touches a directly-created CR. Validation (DNS-label name, one-of repo/image, known plan, port/replica bounds) is shared with that internal path through `store.ValidAppName` + `store.MaxReplicas`, so both agree on what a valid App is.

### 5. The git webhook authenticates by HMAC, outside the OAuth gate

`POST /v1/webhooks/git` closes the push-to-deploy loop. A git host (GitHub/Gitea) cannot present an OAuth2 bearer, so the webhook is mounted **ahead of, and outside,** the auth gate that fronts every other route — its **HMAC-SHA256 signature is its authentication**. It verifies `X-Hub-Signature-256: sha256=<hex>` in constant time against the shared secret `BEX_WEBHOOK_SECRET`; an absent/mismatched signature is **401** with no action, an unset secret makes the endpoint **503** (never accept unsigned pushes). A verified push redeploys every App whose `spec.repo` matches the pushed repository — compared across the payload's clone/ssh/html/api URL forms, each canonicalized (scheme/`user@` stripped, scp-form normalized, trailing `.git` removed) so https and ssh forms of the same repo match — and whose tracked branch matches the pushed ref (`spec.branch`, empty ⇒ `main`).

Because the signature already authorized the call, the webhook is the **one caller of an unexported `redeploy`** that skips the OpenFGA `Authorize` gate (there is no OpenFGA identity on a git-host callback). It stays unexported precisely so `TestAuthzGuardsEveryVerb`'s reflection sweep — which requires every _exported_ service verb to start with `Authorize` — doesn't flag a verb that legitimately authenticates differently. The webhook reads/writes through the raw client, not the authorized `List`/verbs.

### 6. Render consistency, per surface (verified 2026-07-08)

The create surface was checked against the real Render artifacts, per the mandatory per-surface rule ([ADR006-bex-api.md](ADR006-bex-api.md)):

- **REST `POST /v1/services`** (Render public API): matches on the **201** status, top-level `type`/`name`/`repo`/`branch`/`image` (an **object** `{imagePath}`, not a string), `envVars: [{key,value}]`, and the `serviceDetails.{plan, numInstances, healthCheckPath}` nesting — the same location `PATCH`/`GET` already use. Deliberately ignored (bex can't honor them yet, so it doesn't fake them): `ownerId` (single workspace), `region` (single region), `autoDeploy` (bex's push trigger is the webhook, not a poll), and the `serviceDetails` runtime build/start commands (bex auto-detects Dockerfile/CNB). bex-only extensions: `builder`, `port`, `domains`, top-level `plan`.
- **MCP `create_web_service`** (Render's `render-oss/render-mcp-server`): matches the tool name and the `name`/`repo`/`branch`/`plan`/`envVars` args; omits Render's `runtime`/`buildCommand`/`startCommand`/`region`; adds `image`/`port`/`replicas`. `deploy` is a bex extension (Render has no deploy-from-manifest tool).
- **GraphQL `createService`**: a bex extension whose name/shape is **not** confirmed against a live Render dashboard capture — flagged in-code and in [ADR006-bex-api.md](ADR006-bex-api.md), the same caveat `updateServicePlan`/`scaleService` already carry (Render's dashboard even spells restart `restartServer`, so its create mutation name can't be assumed).

## Alternatives considered

- **A bespoke `POST /v1/deploy`** (the original t001) — rejected 2026-07-08. A Render web service already _is_ a repo + config; a second endpoint that turns a repo into a running service is a parallel create surface that drifts from `Core.Create`. Deploy is create with a `repo` body.
- **A dedicated `spec.deployID` / build-revision field for redeploy** — rejected. `spec.restartedAt` already bumps `metadata.generation`, which is exactly what the operator keys a repo rebuild on. A new field would be a second word in the contract meaning the same thing.
- **Route the webhook through the OAuth gate** — impossible: git hosts can't hold a bearer token. HMAC is the standard, and it is a different (payload-integrity) trust model than caller-identity, so the endpoint sits outside the gate by design.
- **Per-app webhook secrets** — deferred. A single shared `BEX_WEBHOOK_SECRET` matches the current single-workspace reality; per-app/per-tenant secrets arrive with tenant onboarding (the store's `apps` rows are where a per-app secret would live). **Update (w2/m9):** the webhook now also accepts a **second** key, `BEX_GITHUB_WEBHOOK_SECRET` — the bex GitHub App's app-wide webhook secret — so app-signed pushes for every installed repo redeploy with no per-repo configuration. A delivery is accepted under either key; the endpoint 503s only when both are unset; GitHub lifecycle events (`ping`/`installation`) are 200 no-ops. Private-repo clones authenticate with a 1h installation token bex-api mints into `spec.cloneSecret` on each deploy trigger ([ADR026-github-integration.md](ADR026-github-integration.md)).
- **Write `Create` through the control-plane store row** — deferred. It needs tenant resolution from the caller identity, which isn't wired on the public surface yet; the direct-CR path delivers the milestone today and the row-backed path grows with the control plane. The projector's label scoping means adding it later doesn't disturb directly-created Apps.

## Consequences

- **"Deploy this" is one verb across all three surfaces.** An agent calls `deploy`, gets back the service object, and polls it to `Running`; a repeat call redeploys; a signed push redeploys hands-free. No dashboard, no `kubectl`. **(w4/m19: `create_web_service` no longer redeploys on repeat — see the Update note in Decision §2 — so this consequence now describes `deploy` specifically, not the structured create tools.)**
- **Idempotent, but no delete yet.** Upsert makes deploy replay-safe. Service **delete** (`DELETE /v1/services/{id}`) is [w2/m4](ADR006-bex-api.md)'s other half, not this ADR.
- **`restartedAt` is overloaded, benignly.** A webhook push to an image-backed App rolls its pods rather than rebuilding (nothing to rebuild) — harmless, and the repo-backed case (the webhook's actual target) rebuilds correctly.
- **The live URL depends on w1/m5.** The handler-level loop (deploy → CR written → webhook verified → `restartedAt` bumped) is tested today; a real `hello-go` build → `*.onbex.co` URL awaits in-cluster builds. [`examples/hello-go/render.yaml`](../examples/hello-go/render.yaml) is the acceptance target.
- **Webhook trust boundary = the shared secret.** Whoever holds `BEX_WEBHOOK_SECRET` can trigger redeploys of matching repos — the same trust shape as the other platform secrets in `.env`. HMAC gives payload integrity + authentication, not replay protection; a re-sent valid push simply redeploys again (idempotent), so replay is a no-op here.

## Verification

1. **Deploy** — MCP `deploy` with `examples/hello-go/render.yaml` (+ its repo) → an `App` CR is written; poll `get_service` until `status.url` is live; `curl` it (needs w1/m5 builds for the real URL; the CR write + mapping is unit-tested now).
2. **Redeploy on push** — `POST /v1/webhooks/git` with a valid `X-Hub-Signature-256` for that repo/branch → the matching App's `spec.restartedAt` bumps and the revision rolls; an invalid/absent signature → 401, no change; unset secret → 503.
3. **Idempotence** — a second `deploy` for the same name updates the existing App (redeploy), never a duplicate.
