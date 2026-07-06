# w4 · m1 — Platform auth: Ory Kratos + Hydra on the cluster (+ ADR)

**Worker:** worker4 **Goal:** Kratos (identity) and Hydra (OAuth2/OIDC) run on the cluster backed by their own CNPG Postgres clusters, exposed via Traefik with certs, credentials sourced from `.env` (never committed); the decision recorded as an ADR in `docs/auth.md`. **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on |
| ---- | ------------------------------------------------------------------------ | --- | ---------- |
| t001 | CNPG clusters `kratos-db` + `hydra-db` (base manifests + local sizing)   | 30m | —          |
| t002 | Secret bootstrap script: k8s Secrets from `.env`, never committed        | 25m | t001       |
| t003 | Argo Application for Kratos (pinned chart, values, identity schema)      | 30m | t002       |
| t004 | Argo Application for Hydra (pinned chart, values, admin/public split)    | 30m | t002       |
| t005 | Traefik IngressRoutes + cert-manager for Kratos/Hydra public endpoints   | 25m | t003, t004 |
| t006 | E2E verify on local mock cluster: identity + client_credentials token    | 25m | t005       |
| t007 | ADR `docs/auth.md` + docs index entry in root `CLAUDE.md`                | 30m | t006       |
| t008 | Simplify — run `/simplify` over the code this milestone changed          | 20m | t007       |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped | 30m | t007       |

## Definition of done

On the local mock cluster, Kratos and Hydra pods are healthy against their CNPG databases; an identity can be created via the Kratos admin API and an OAuth2 `client_credentials` token issued by Hydra, and both survive a pod restart (state lives in Postgres); `docs/auth.md` exists and is indexed in root `CLAUDE.md`; no secret material is committed to git.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-05 (plan: pm-brainstorm-add-w4-workstream) — user request to add Kratos + Hydra + DBs and an auth ADR.
- **Goal linkage:** vision roadmap #1 (Postgres control plane with tenants/accounts — w1/m2 t004) and pillars 3–4 (agents need per-client credentials, not one shared token).
- **Expected outcome:** a running identity + OAuth2 provider the control plane and bex-api can build on.
- **Why now:** w1/m2 t004 (accounts/auth tables) and w2/m2 (deploy-from-chat) both presuppose real identity/token infrastructure; the single static bearer token is the blocker for multi-tenancy.
