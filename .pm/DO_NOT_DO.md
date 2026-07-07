# DO NOT DO — roadmap anti-goals and guardrails

Use this file as a hard constraint when running `/pm-brainstorm` and `/pm`. If a proposed milestone/task conflicts with any item below, reject it.

## Anti-goals

- Do not propose roadmap work that is not clearly tied to bex's project goals/pillars (`docs/vision.md`) or current platform roadmap intent.
- Do not create milestones that are vague, non-testable, or missing observable outcomes.
- Do not create milestones for sub-hour work; keep those as inbox notes (`wN/NNN.md`).
- Do not add "nice-to-have" work that has no clear sequence/dependency/risk rationale for why it must happen now.
- Do not duplicate existing milestones/tasks without a clear gap analysis and replacement intent.
- Do not propose work that conflicts with established architectural boundaries (bex product code vs platform GitOps responsibilities).
- Do not include tasks that cannot define concrete files/resources/systems they touch.
- Do not treat speculative ideas as committed roadmap items without explicit source context.
- **Do not OAuth2-ize the first-party dashboard.** The dashboard authenticates with its Kratos session (HttpOnly cookie, same-site) — never make it a Hydra OAuth2 client, and never store OAuth access/refresh tokens (or Kratos session tokens) in browser-readable storage. IETF `draft-ietf-oauth-browser-based-apps` ranks browser-held tokens last (BFF/cookie-session first); Ory's own guidance is "first-party apps use Kratos sessions; Hydra is for third-party/machine clients". Local-dev cross-origin is a dev-workflow problem — solve it with the Vite dev proxy (`dashboard/.env.example`, same-origin tunnel), not by changing the production auth architecture. (Learned from w4/m9, reverted 2026-07-07.)
- **Do not hand-build a Hydra login/consent provider.** Kratos's native `oauth2_provider.url` integration accepts Hydra login challenges itself (the login UI only passes `login_challenge` through); at most a headless consent acceptor is needed. A custom provider app (in bex-api or the dashboard) re-implements what Kratos ships. (Same w4/m9 lesson; the future agent OAuth2.1 work is `w2/001.md`.)

## Minimum bar for a meaningful milestone

Every proposed milestone must include:

- goal linkage (which project goal/pillar it advances),
- expected outcome (observable impact),
- why now (dependency/risk/sequence rationale),
- definition of done with testable end state.
