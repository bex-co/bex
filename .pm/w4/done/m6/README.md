# w4 · m6 — Tenant secrets: env-vars API backed by OpenBao, injected into Apps

**Worker:** worker4 **Goal:** the first end-to-end tenant-credential path — a tenant (API key) sets env vars through Render-compatible endpoints, the values live in OpenBao (m5) under that service's path, and the operator materializes them into the running app's Deployment, closing the gap where Apps today receive no configuration beyond `PORT`. Secrets verbs are permission-checked through m4's `Checker`. **Status:** done — DoD met; `secrets-verify.sh` green end-to-end on the mock cluster (value in OpenBao → materialized → served; tuple-less key 403; unset ⇒ 503), `make test` green (operator + backend).

**Follow-up (post-DoD):** completed Render REST parity — added the two single-key endpoints Render has that t003 hadn't scoped: `GET /v1/services/{id}/env-vars/{key}` (retrieve one, bare `{key,value}`) and `PUT /v1/services/{id}/env-vars/{key}` (add/update one, body `{value}`, merge-not-replace), across REST/GraphQL (`envVar`/`setEnvVar`)/MCP (`get_env_var`/`set_env_var`), with tests. bex now covers all 5 of Render's env-var endpoints; the one deliberate divergence (bex rolls the pods on write; Render does not auto-deploy) and the omissions (cursor pagination, `generateValue`, env groups, secret files) are documented in `docs/ADR006-bex-api.md`.

**Follow-up 2 (GraphQL alignment):** verified bex's GraphQL against Render's live dashboard (Playwright capture: Render's env page fires `serviceEnvVarKeys` = `service(id){ envVarKeys{ id key } }`, keys-only, values on demand). Reshaped bex's GraphQL to match: env-var reads now **nest under the `Service` type** as `envVarKeys` (keys-only, each with an `id`==key) + `envVar(key)` (per-key value, mirroring "Show secret"), with a `service(id)` query alias so the dashboard query resolves byte-for-byte; removed the flat top-level `envVars`/`envVar`. REST stays the public-API-shaped surface, GraphQL the dashboard-shaped one — same `Core` verbs, documented in `docs/ADR006-bex-api.md`. (Secret files / env groups are Render features bex doesn't implement; Render's mutation names weren't captured — doing so would mutate a live service — so bex keeps its own `setEnvVars`/`setEnvVar`/`deleteEnvVar`.)

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on | status        |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- | ------------- |
| t001 | App CRD env plumbing: `spec.env` + secret-ref support in the kubernetes runtime                               | 40m | — (w4/m5)  | — **DONE**    |
| t002 | Core `SecretStore` seam + OpenBao KV client (`BEX_OPENBAO_URL`, k8s auth login)                                | 40m | t001       | — **DONE**    |
| t003 | Render-compatible REST `GET/PUT/DELETE /v1/services/{id}/env-vars` + GraphQL + MCP adapters                    | 45m | t002       | — **DONE**    |
| t004 | Materialize on write: sync OpenBao values into a per-app k8s Secret via `envFrom`; change triggers rollout     | 40m | t003       | — **DONE**    |
| t005 | Authorization: secrets verbs enforced through m4's `Checker`; values never leak to logs/responses              | 25m | t004       | — **DONE**    |
| t006 | E2E on mock cluster: key → PUT env-vars → app serves new value; no-tuple key 403; env unset ⇒ 503              | 35m | t005       | — **DONE**    |
| t007 | Docs: `docs/ADR006-bex-api.md` + `docs/ADR013-secrets.md` product section + env tables                                       | 20m | t006       | — **DONE**    |
| t008 | Simplify — run `/simplify` over the code this milestone changed                                                | 20m | t007       | — **DONE**    |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                                       | 30m | t007       | — **DONE**    |

## Definition of done

On the local mock cluster with m5 running: a tenant (API key) sets env vars through the Render-shaped REST endpoint; the values are stored in OpenBao under that service's path (verifiable with `bao kv get`), materialized into the app's Deployment, and observable in the running app's HTTP response after the automatic rollout; a key without manage permission gets 403 via the OpenFGA checker; with `BEX_OPENBAO_URL` unset, the secrets endpoints 503 and everything else is byte-for-byte today's behavior (`make test` green unchanged); the Core seam is covered by unit tests with a fake store.

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (user request: "add openbao so tenants can store their credentials"); needs w4/m5 (substrate) and w4/m4 (`Checker` seam).
- **Goal linkage:** pillars 1 (Render-compatible API — env-vars is a real Render endpoint agents already call) and 4 (deploy-from-chat needs credentials delivery); roadmap #1 (multi-tenant control plane).
- **Expected outcome:** the first end-to-end tenant-credential path: API in, OpenBao at rest, running container out — Apps today cannot receive _any_ configuration beyond `PORT`.
- **Why now:** it is the consumer that justifies m5, and it lands on the two seams w4 just built (`api.IdentityFrom` from m3, `Checker` from m4) while they are fresh; deferring it means the control-plane work (w1/m2) would design tenants without a credential story.
