# w4 — Auth & identity (worker4)

**Worker:** worker4 w1 builds the platform, w2 makes it AI-native, w3 makes it observable — w4 makes it multi-tenant-secure: real identities (Ory Kratos) and OAuth2 tokens (Ory Hydra) replacing the single static `BEX_API_TOKEN`. Ordered by dependency: deploy the auth substrate first (GitOps side), then wire bex-api to it (product side).

## Milestones

- [x] **m1** — Platform auth: Ory Kratos + Hydra on the cluster (+ ADR) (9 tasks) ← from brainstorm 2026-07-05
- [ ] **m2** — bex-api auth: Hydra introspection + Kratos sessions (6 tasks) ← from brainstorm 2026-07-05, needs m1
