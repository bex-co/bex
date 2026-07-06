# w4 · m6 — Tenant secrets: env-vars API backed by OpenBao, injected into Apps

**Worker:** worker4 **Goal:** the first end-to-end tenant-credential path — a tenant (API key) sets env vars through Render-compatible endpoints, the values live in OpenBao (m5) under that service's path, and the operator materializes them into the running app's Deployment, closing the gap where Apps today receive no configuration beyond `PORT`. Secrets verbs are permission-checked through m4's `Checker`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                        | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | App CRD env plumbing: `spec.env` + secret-ref support in the kubernetes runtime                               | 40m | — (w4/m5)  |
| t002 | Core `SecretStore` seam + OpenBao KV client (`BEX_OPENBAO_URL`, k8s auth login)                                | 40m | t001       |
| t003 | Render-compatible REST `GET/PUT/DELETE /v1/services/{id}/env-vars` + GraphQL + MCP adapters                    | 45m | t002       |
| t004 | Materialize on write: sync OpenBao values into a per-app k8s Secret via `envFrom`; change triggers rollout     | 40m | t003       |
| t005 | Authorization: secrets verbs enforced through m4's `Checker`; values never leak to logs/responses              | 25m | t004       |
| t006 | E2E on mock cluster: key → PUT env-vars → app serves new value; no-tuple key 403; env unset ⇒ 503              | 35m | t005       |
| t007 | Docs: `docs/bex-api.md` + `docs/secrets.md` product section + env tables                                       | 20m | t006       |
| t008 | Simplify — run `/simplify` over the code this milestone changed                                                | 20m | t007       |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                                       | 30m | t007       |

## Definition of done

On the local mock cluster with m5 running: a tenant (API key) sets env vars through the Render-shaped REST endpoint; the values are stored in OpenBao under that service's path (verifiable with `bao kv get`), materialized into the app's Deployment, and observable in the running app's HTTP response after the automatic rollout; a key without manage permission gets 403 via the OpenFGA checker; with `BEX_OPENBAO_URL` unset, the secrets endpoints 503 and everything else is byte-for-byte today's behavior (`make test` green unchanged); the Core seam is covered by unit tests with a fake store.

## Source + Goal linkage

- **Source:** /pm-brainstorm 2026-07-06 (user request: "add openbao so tenants can store their credentials"); needs w4/m5 (substrate) and w4/m4 (`Checker` seam).
- **Goal linkage:** pillars 1 (Render-compatible API — env-vars is a real Render endpoint agents already call) and 4 (deploy-from-chat needs credentials delivery); roadmap #1 (multi-tenant control plane).
- **Expected outcome:** the first end-to-end tenant-credential path: API in, OpenBao at rest, running container out — Apps today cannot receive _any_ configuration beyond `PORT`.
- **Why now:** it is the consumer that justifies m5, and it lands on the two seams w4 just built (`api.IdentityFrom` from m3, `Checker` from m4) while they are fresh; deferring it means the control-plane work (w1/m2) would design tenants without a credential story.
