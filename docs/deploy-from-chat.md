# ADR: deploy-from-chat — create-as-deploy + HMAC push-to-deploy

**Status:** accepted — implemented 2026-07-08 in bex-api (`lego/backend/internal/apps`: `Create`/`Deploy`/webhook, over REST/GraphQL/MCP), unit- and integration-tested. The **live** "repo → https URL" leg needs in-cluster builds ([w1/m5](../.pm/w1/m5/README.md)); until that lands the loop is proven at the handler level, not end-to-end on a cluster.

## Context

[docs/vision.md](vision.md) pillar 4 is "deploy-from-chat": one API call takes a repo + its `bex.yml` to a live https URL, so "deploy this" is a single agent action, and a later git push redeploys. bex already has the mechanism — the operator builds `App.spec.repo` from git and serves it; [restart-suspend-and-resume.md](restart-suspend-and-resume.md) established that a product action is **a word in the `App` CR contract**, not an operator endpoint. What was missing was the _product verb_ on bex-api and the _push trigger_.

Two shapes were on the table for the verb. The original `.pm/w2/m2/t001` proposed a bespoke `POST /v1/deploy`. But [w2/m4](bex-api.md) was already adding Render's own create surface (`POST /v1/services` / `create_web_service` over one `Core.Create`) — and a Render web service _is_ a repo + config, which is exactly what "deploy" carries. A second endpoint that also turns a repo into a running service would be a parallel surface that drifts from the create one. So t001 was **amended 2026-07-08** to ride `Core.Create` instead. This ADR records the resulting decisions, because they set the precedent for the next verbs (rollback, deploy history — [w2/m5](../.pm/w2/m5/README.md)).

The rule this works within (from [restart-suspend-and-resume.md](restart-suspend-and-resume.md), [control-plane.md](control-plane.md)): product action → **App CR spec** (intent) → operator converges. bex-api writes intent and never calls the operator; the operator keys a rebuild on `metadata.generation` changing (a genuine spec bump, not suspended, repo-backed → re-run build-from-git).

## Decision

### 1. Deploy _is_ create — no bespoke endpoint

Deploy-from-chat rides the same `Core.Create` every other client uses. Two entry points, one verb:

- **`deploy {repo, branch?, bexYaml}`** (MCP) — the agent-facing verb: it parses the render.yaml-shaped `bex.yml`, maps its first app onto a `CreateRequest`, and calls `Create`. The `repo`/`branch` args override the manifest (an agent that already knows the checkout needn't duplicate it).
- **`create_web_service` / `POST /v1/services` / `createService`** — the structured create surface (a `repo` or a prebuilt `image`) across MCP/REST/GraphQL.

The `bex.yml` → spec field mapping is the **same one [`scripts/app-apply.sh`](../scripts/app-apply.sh) uses** (`domains[0]`→`spec.host`, `envVars[]`→`spec.env[]`, `type: web`→exposed, `type: private`→in-cluster-only), so a manifest that applies with the script deploys through the API. There is no `POST /v1/deploy`.

### 2. Create is an upsert; a repeat deploy is a redeploy

`Create` is **create-or-update**: a new name creates the `App` CR; an existing name re-applies the request's spec fields and bumps `spec.restartedAt`. So calling `deploy` twice for the same service redeploys it — never a duplicate — which is exactly "push the button again" from a chat. The update path re-applies only create-owned fields and leaves operator/other-feature fields alone (`EnvFromSecret` from the [secrets](secrets.md) env-vars API, `Suspended`, `IdleTTLSeconds`, `AutoDeploy`) — the same discipline the control-plane projector's `applyOwnedSpec` follows.

### 3. Redeploy reuses `spec.restartedAt` — no new contract word

Redeploy needs the operator to roll a fresh revision. The operator already keys that on `metadata.generation` changing, and [restart-suspend-and-resume.md](restart-suspend-and-resume.md) already added `spec.restartedAt` as the verb-as-timestamp that bumps generation. For a **repo-backed** App a generation bump invalidates the cached `Status.Image` and re-runs build-from-git (a new revision); for an **image-backed** App it is a rolling restart of the same image. So redeploy = stamp `spec.restartedAt` — no `spec.deployID`/revision field invented, the same field the upsert and the webhook both write.

### 4. Create writes the App CR directly (the hand-applied path)

The public create surface has no tenant context, so it writes the `App` CR **directly** (like `scripts/app-apply.sh`), not through a control-plane `apps` row. The row-backed, multi-tenant create — which needs a `tenantId` and mints a `<tenant>-<app>` CR name — stays the internal control-plane API's job ([control-plane.md](control-plane.md), `store` `POST /v1/apps`). The two coexist safely: the projector lists and deletes **only** CRs carrying its `app.kubernetes.io/managed-by: bex-controlplane` label, so it never touches a directly-created CR. Validation (DNS-label name, one-of repo/image, known plan, port/replica bounds) is shared with that internal path through `store.ValidAppName` + `store.MaxReplicas`, so both agree on what a valid App is.

### 5. The git webhook authenticates by HMAC, outside the OAuth gate

`POST /v1/webhooks/git` closes the push-to-deploy loop. A git host (GitHub/Gitea) cannot present an OAuth2 bearer, so the webhook is mounted **ahead of, and outside,** the auth gate that fronts every other route — its **HMAC-SHA256 signature is its authentication**. It verifies `X-Hub-Signature-256: sha256=<hex>` in constant time against the shared secret `BEX_WEBHOOK_SECRET`; an absent/mismatched signature is **401** with no action, an unset secret makes the endpoint **503** (never accept unsigned pushes). A verified push redeploys every App whose `spec.repo` matches the pushed repository — compared across the payload's clone/ssh/html/api URL forms, each canonicalized (scheme/`user@` stripped, scp-form normalized, trailing `.git` removed) so https and ssh forms of the same repo match — and whose tracked branch matches the pushed ref (`spec.branch`, empty ⇒ `main`).

Because the signature already authorized the call, the webhook is the **one caller of an unexported `redeploy`** that skips the OpenFGA `Authorize` gate (there is no OpenFGA identity on a git-host callback). It stays unexported precisely so `TestAuthzGuardsEveryVerb`'s reflection sweep — which requires every _exported_ service verb to start with `Authorize` — doesn't flag a verb that legitimately authenticates differently. The webhook reads/writes through the raw client, not the authorized `List`/verbs.

### 6. Render consistency, per surface (verified 2026-07-08)

The create surface was checked against the real Render artifacts, per the mandatory per-surface rule ([bex-api.md](bex-api.md)):

- **REST `POST /v1/services`** (Render public API): matches on the **201** status, top-level `type`/`name`/`repo`/`branch`/`image` (an **object** `{imagePath}`, not a string), `envVars: [{key,value}]`, and the `serviceDetails.{plan, numInstances, healthCheckPath}` nesting — the same location `PATCH`/`GET` already use. Deliberately ignored (bex can't honor them yet, so it doesn't fake them): `ownerId` (single workspace), `region` (single region), `autoDeploy` (bex's push trigger is the webhook, not a poll), and the `serviceDetails` runtime build/start commands (bex auto-detects Dockerfile/CNB). bex-only extensions: `builder`, `port`, `domains`, top-level `plan`.
- **MCP `create_web_service`** (Render's `render-oss/render-mcp-server`): matches the tool name and the `name`/`repo`/`branch`/`plan`/`envVars` args; omits Render's `runtime`/`buildCommand`/`startCommand`/`region`; adds `image`/`port`/`replicas`. `deploy` is a bex extension (Render has no deploy-from-manifest tool).
- **GraphQL `createService`**: a bex extension whose name/shape is **not** confirmed against a live Render dashboard capture — flagged in-code and in [bex-api.md](bex-api.md), the same caveat `updateServicePlan`/`scaleService` already carry (Render's dashboard even spells restart `restartServer`, so its create mutation name can't be assumed).

## Alternatives considered

- **A bespoke `POST /v1/deploy`** (the original t001) — rejected 2026-07-08. A Render web service already _is_ a repo + config; a second endpoint that turns a repo into a running service is a parallel create surface that drifts from `Core.Create`. Deploy is create with a `repo` body.
- **A dedicated `spec.deployID` / build-revision field for redeploy** — rejected. `spec.restartedAt` already bumps `metadata.generation`, which is exactly what the operator keys a repo rebuild on. A new field would be a second word in the contract meaning the same thing.
- **Route the webhook through the OAuth gate** — impossible: git hosts can't hold a bearer token. HMAC is the standard, and it is a different (payload-integrity) trust model than caller-identity, so the endpoint sits outside the gate by design.
- **Per-app webhook secrets** — deferred. A single shared `BEX_WEBHOOK_SECRET` matches the current single-workspace reality; per-app/per-tenant secrets arrive with tenant onboarding (the store's `apps` rows are where a per-app secret would live).
- **Write `Create` through the control-plane store row** — deferred. It needs tenant resolution from the caller identity, which isn't wired on the public surface yet; the direct-CR path delivers the milestone today and the row-backed path grows with the control plane. The projector's label scoping means adding it later doesn't disturb directly-created Apps.

## Consequences

- **"Deploy this" is one verb across all three surfaces.** An agent calls `deploy` (or `create_web_service`), gets back the service object, and polls it to `Running`; a repeat call redeploys; a signed push redeploys hands-free. No dashboard, no `kubectl`.
- **Idempotent, but no delete yet.** Upsert makes deploy replay-safe. Service **delete** (`DELETE /v1/services/{id}`) is [w2/m4](bex-api.md)'s other half, not this ADR.
- **`restartedAt` is overloaded, benignly.** A webhook push to an image-backed App rolls its pods rather than rebuilding (nothing to rebuild) — harmless, and the repo-backed case (the webhook's actual target) rebuilds correctly.
- **The live URL depends on w1/m5.** The handler-level loop (deploy → CR written → webhook verified → `restartedAt` bumped) is tested today; a real `hello-go` build → `*.onbex.co` URL awaits in-cluster builds. [`examples/hello-go/bex.yml`](../examples/hello-go/bex.yml) is the acceptance target.
- **Webhook trust boundary = the shared secret.** Whoever holds `BEX_WEBHOOK_SECRET` can trigger redeploys of matching repos — the same trust shape as the other platform secrets in `.env`. HMAC gives payload integrity + authentication, not replay protection; a re-sent valid push simply redeploys again (idempotent), so replay is a no-op here.

## Verification

1. **Deploy** — MCP `deploy` with `examples/hello-go/bex.yml` (+ its repo) → an `App` CR is written; poll `get_service` until `status.url` is live; `curl` it (needs w1/m5 builds for the real URL; the CR write + mapping is unit-tested now).
2. **Redeploy on push** — `POST /v1/webhooks/git` with a valid `X-Hub-Signature-256` for that repo/branch → the matching App's `spec.restartedAt` bumps and the revision rolls; an invalid/absent signature → 401, no change; unset secret → 503.
3. **Idempotence** — a second `deploy` for the same name updates the existing App (redeploy), never a duplicate.
