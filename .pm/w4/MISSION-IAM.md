# w4 MISSION — IAM (worker4)

You are **worker4**. Your mission is IAM for bex: replace the single static `BEX_API_TOKEN` with real identities and per-client, revocable credentials — **Ory Kratos** (identity, sessions) + **Ory Hydra** (OAuth2/OIDC tokens) — so the platform can become multi-tenant (roadmap #5, `.pm/GOAL.md`) and every agent surface gets its own credentials instead of one shared secret.

## Scope

- **In:** the auth substrate on the cluster (CNPG-backed Kratos + Hydra, Argo CD apps, Traefik + cert-manager exposure, secret bootstrap from `.env`), bex-api auth middleware (Hydra introspection, Kratos sessions, `BEX_AUTH_MODE` flag), the ADR `docs/auth.md`.
- **Out:** the Postgres control-plane schema (w1), agent/MCP surfaces (w2), observability (w3). Consume their work; don't build it. Respect the product-vs-GitOps boundary (`docs/go-and-gitops.md`): platform auth deploys live in `deploy/gitops/`, token validation lives in `operator/` bex-api code.

## Order of work

1. **m1 — Platform auth: Kratos + Hydra on the cluster (+ ADR).** Deploy-only; verify on the local mock cluster (identity via Kratos admin API, `client_credentials` token from Hydra, state survives pod restart).
2. **m2 — bex-api auth: Hydra introspection + Kratos sessions.** Blocked on m1. Static-token mode stays as a flagged fallback.

Work tasks in the dependency order in each milestone `README.md`. When something is done, follow the done-folder rule (`.pm/CLAUDE.md`): sync frontmatter, milestone README, workstream README checkbox, and move the file/dir into `done/`.

## Hard rules

- Never commit or push unless the user runs `/ship`.
- No secret material in git — credentials come from `.env` via the bootstrap script; never print or commit `.env` / `*.kubeconfig`.
- `.pm/DO_NOT_DO.md` is a hard constraint for any new work you propose.
- Markdown changes: run `npx prettier@3.4.2 --write` before finishing.

## Definition of mission success

With `BEX_AUTH_MODE=ory` on the local mock cluster: an identity exists in Kratos, Hydra issues a `client_credentials` token, that token authenticates bex-api REST and GraphQL calls, an invalid/expired token gets 401, legacy static-token mode still works when flagged, and `docs/auth.md` records the decision and is indexed in root `CLAUDE.md`.
