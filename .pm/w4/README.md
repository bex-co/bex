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
- [x] **m7** — Email flows: Kratos courier + recovery/verification live (8 tasks) ← from brainstorm 2026-07-06, independent of m4–m6 (closing tasks retrofitted 2026-07-09) — done 2026-07-12 (recovery + verification proven e2e via `scripts/auth-mail-e2e.sh`; courier/relay wired prod + local; `auth.verification` dashboard page filed as inbox `w4/008`), moved to `done/m7/`
- [x] **m8** — API keys in the dashboard (settings surface) (6 tasks) ← from brainstorm 2026-07-06, coordinates with m4's checker; inbox `004`'s key-metadata half was NOT done alongside t001/t002 — still open
- [x] **m9** — OAuth 2.1 provider for agents: one dashboard login, first-party sessions + third-party clients (7 tasks) ← promoted from w4/001 (user request 2026-07-07), needs m1 + m2 — done 2026-07-08 (e2e-verified incl. real-browser login pass), moved to `done/m9/`
- [x] **m10** — Audit log: every Core write recorded with caller identity (8 tasks) ← from `/pm-brainstorm tasks for w4` 2026-07-08 (closing tasks retrofitted 2026-07-09) — done 2026-07-11 (verified via unit + real-Postgres integration tests; live mock-cluster smoke not run — no cluster in this environment), moved to `done/m10/`
- [ ] **m11** — MFA: TOTP + passkeys via Kratos (8 tasks) ← promoted from `002` 2026-07-08 (its w1/m2 revisit condition met; closing tasks retrofitted 2026-07-09)
- [x] **m12** — Workspace members & roles (Render team surface) (9 tasks) ← from `/pm-brainstorm tasks for w4` 2026-07-08, needs w1/m9 + w4/m7 (closing tasks retrofitted 2026-07-09)
- [x] **m13** — API-key hygiene: token TTL + key metadata (7 tasks) ← promoted from `004` 2026-07-09 (its m8 pairing plan didn't happen); done 2026-07-09
- [x] **m14** — Audit log in the dashboard: Settings → Audit Log panel over the m10 API (10 tasks) ← user request 2026-07-11, needs m10 (done) — done 2026-07-11 (`yarn test`/`yarn lint` green; IA-placement drift vs Render found in parity check, filed as `007`), moved to `done/m14/`
- [x] **m15** — Settings → Security & Compliance grouping (move Audit Log) (6 tasks) ← promoted from `007` 2026-07-12 (user resolved the IA-placement decision as "yes, move it"), coordinates with `006` (session mgmt) + m11 (MFA) on the same Settings→Security surface — done 2026-07-12 (real-app verified: Audit Log card renders inside the Security & Compliance region; `yarn test` 613 green), moved to `done/m15/`
- [x] **m16** — OAuth consent screen for third-party agent clients (9 tasks) ← from `/pm-brainstorm more for w4` 2026-07-12 (ADR012 §7 "consent UI is future work" + ADR025's operator-blessing dead end), follow-on to m9; anti-goal tension (DO_NOT_DO "headless consent acceptor") resolved by user acceptance 2026-07-12 — done 2026-07-12 (`scripts/auth-oauth21-e2e.sh` exit 0 over 12 legs: unblessed DCR client → consent page → approve → working `/mcp` token, deny → `access_denied`, remembered re-authorize skips the page, trusted client still headless; `yarn test` 675 green; a login-path gap found on the way filed as `010`), moved to `done/m16/`
- [ ] **m17** — Login-flow correctness: signed-in OAuth authorize + aal2 step-up altitude (8 tasks) ← promoted from `010` (+ `009` folded in) 2026-07-12, follow-on to m16; fixes the ADR025 signed-in connect path
- [ ] **m18** — Account access control: connected agents + active sessions (8 tasks) ← from `/pm-brainstorm for w4` 2026-07-12, m16 follow-on (remembered consent shipped with no revocation surface); folds `006`; independent of m17 (do m17 first — live user-facing break)

## Suggested execution order (2026-07-09 brainstorm)

**m7 → (m10 ∥ m11) → m13 (small, anytime) → m12.** m7 gates m12 (invites need the courier) and closes a live lockout risk; m10's one-interception-point argument strengthens as w2/m4–m5 add write verbs; m11 before real tenants means no forced-enrollment migration; m12 stays gated on w1/m9 + m7.

## Inbox

_(empty)_

> `002.md` (MFA) promoted to **m11** 2026-07-08; `004.md` (credential hygiene) promoted to **m13** 2026-07-09; `003.md` (GitHub social login) implemented directly 2026-07-11 (Kratos `oidc` + Dex e2e, docs/ADR012-auth.md §10); `007.md` (Audit Log IA placement) promoted to **m15** 2026-07-12; `010.md` promoted to **m17** with `009.md` folded in as its aal2-altitude task, and `006.md` (session management) folded into **m18** as its active-sessions task, 2026-07-12 — notes moved to `done/`.
