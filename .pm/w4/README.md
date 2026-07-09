# w4 — Auth & identity (worker4)

**Worker:** worker4 w1 builds the platform, w2 makes it AI-native, w3 makes it observable — w4 makes it multi-tenant-secure: real identities (Ory Kratos) and OAuth2 tokens (Ory Hydra) replacing the single static `BEX_API_TOKEN`. Ordered by dependency: deploy the auth substrate first (GitOps side), then wire bex-api to it (product side).

## Milestones

- [x] **m1** — Platform auth: Ory Kratos + Hydra on the cluster (+ ADR) (9 tasks) ← from brainstorm 2026-07-05
- [x] **m2** — bex-api auth: Hydra introspection + Kratos sessions (6 tasks) ← from brainstorm 2026-07-05, needs m1
- [x] **m3** — API keys replace the static token (7 tasks) ← user decision 2026-07-06, needs m2
- [x] **m4** — Authorization: OpenFGA over the auth substrate (8 tasks) ← promoted from w4/001, needs m3
- [x] **m5** — Platform secrets: OpenBao on the cluster (+ ADR) (8 tasks) ← from brainstorm 2026-07-06, parallel with m4
- [x] **m6** — Tenant secrets: env-vars API backed by OpenBao, injected into Apps (9 tasks) ← from brainstorm 2026-07-06, needs m4 + m5
- [x] **m6.5** — Env vars in the dashboard: Render-style Environment tab wired to the m6 API (7 tasks) ← user request 2026-07-07, needs m6 + w5/m5 (service-detail IA)
- [ ] **m7** — Email flows: Kratos courier + recovery/verification live (8 tasks) ← from brainstorm 2026-07-06, independent of m4–m6 (closing tasks retrofitted 2026-07-09)
- [x] **m8** — API keys in the dashboard (settings surface) (6 tasks) ← from brainstorm 2026-07-06, coordinates with m4's checker; inbox `004`'s key-metadata half was NOT done alongside t001/t002 — still open
- [x] **m9** — OAuth 2.1 provider for agents: one dashboard login, first-party sessions + third-party clients (7 tasks) ← promoted from w4/001 (user request 2026-07-07), needs m1 + m2 — done 2026-07-08 (e2e-verified incl. real-browser login pass), moved to `done/m9/`
- [ ] **m10** — Audit log: every Core write recorded with caller identity (8 tasks) ← from `/pm-brainstorm tasks for w4` 2026-07-08 (closing tasks retrofitted 2026-07-09)
- [ ] **m11** — MFA: TOTP + passkeys via Kratos (8 tasks) ← promoted from `002` 2026-07-08 (its w1/m2 revisit condition met; closing tasks retrofitted 2026-07-09)
- [ ] **m12** — Workspace members & roles (Render team surface) (9 tasks) ← from `/pm-brainstorm tasks for w4` 2026-07-08, needs w1/m9 + w4/m7 (closing tasks retrofitted 2026-07-09)
- [x] **m13** — API-key hygiene: token TTL + key metadata (7 tasks) ← promoted from `004` 2026-07-09 (its m8 pairing plan didn't happen); done 2026-07-09

## Suggested execution order (2026-07-09 brainstorm)

**m7 → (m10 ∥ m11) → m13 (small, anytime) → m12.** m7 gates m12 (invites need the courier) and closes a live lockout risk; m10's one-interception-point argument strengthens as w2/m4–m5 add write verbs; m11 before real tenants means no forced-enrollment migration; m12 stays gated on w1/m9 + m7.

## Inbox

- `003.md` — GitHub social login via Kratos `oidc` — still parked on its two blockers (OAuth-app ownership for self-hosters; local E2E story)
- `006.md` — Account session management (Kratos `/sessions` list + sign-out-everywhere card in Settings→Security) — sub-hour; ride alongside m11 ← `/pm-brainstorm for w4` 2026-07-09
> `002.md` (MFA) promoted to **m11** 2026-07-08; `004.md` (credential hygiene) promoted to **m13** 2026-07-09; notes moved to `done/`.
