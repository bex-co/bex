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
- **Do not OAuth2-ize the first-party dashboard.** The dashboard authenticates with its Kratos session (HttpOnly cookie, same-site) — never make it a Hydra OAuth2 client, and never store OAuth access/refresh tokens (or Kratos session tokens) in browser-readable storage. IETF `draft-ietf-oauth-browser-based-apps` ranks browser-held tokens last (BFF/cookie-session first); Ory's own guidance is "first-party apps use Kratos sessions; Hydra is for third-party/machine clients". Local-dev cross-origin is a dev-workflow problem — solve it with the Vite dev proxy (`dashboard/.env.example`, same-origin tunnel), not by changing the production auth architecture. (Learned from a reverted, never-committed dashboard-as-OAuth2-client attempt, 2026-07-07; the corrected **provider-side** work is `w4/m9`, which keeps the dashboard on sessions.)
- **Do not hand-build a Hydra login/consent provider.** Kratos's native `oauth2_provider.url` integration accepts Hydra login challenges itself (the login UI only passes `login_challenge` through); at most a headless consent acceptor is needed. A custom provider app (in bex-api or the dashboard) re-implements what Kratos ships. (Same lesson from the reverted 2026-07-07 attempt; `w4/m9` is the corrected design.)
- **Do not build a "Link database to service" picker in the dashboard** (an "Add from database" env-var flow that inserts a managed database's connection string into a service's environment). Render has no such dashboard feature — its env editor offers only **+ Add Environment Variable** and **Add from .env** (verified against render.com/docs/configure-environment-variables, 2026-07-08); the Render flow is copy the Internal Database URL from the database page, paste it into the service's env vars, and the m8 Databases page (connection-info reveal + copy buttons) plus the existing Environment tab already cover that. Render's only first-class DB→service linkage is the Blueprint `fromDatabase` field (IaC) — if bex ever mirrors that, it's `bex.yml`/backend spec work (w1), not a dashboard picker. (Withdrawn from `/pm-brainstorm for w5`, 2026-07-08.)
- **Do not build hosted agent sandboxes (E2B-compatible execution environments) for now — pillar 5 is off the roadmap.** User decision 2026-07-08 (follow-up to `/pm-brainstorm for w2`): the Render-alternative core — in-cluster builds (w1/m5), tenancy + enforced authz (w1/m9), elasticity (w1/m3), Render parity (w1/m13, w2/m4–m5) — is unfinished, and sandboxes are a second product surface competing for the same worker capacity; the mechanism is also single-host-only today (opensandbox Docker runtime `:8077`; the k8s snapshot path is blocked — docs/sandboxes.md D5/D6). Do not propose sandbox milestones, tasks, or adjacent UX/pricing work (e.g. the GitHub-premium-request consumption model discussed 2026-07-08) unless the user explicitly re-opens pillar 5. `w2/m3` removed 2026-07-08; the design record stays in docs/sandboxes.md (ADR, status: proposed) so a future re-open starts from the settled architecture, not from scratch.
- **Do not use vcluster (or any per-tenant virtual control plane) as a tenant isolation tier.** It only isolates the control-plane/API axis — a surface bex's Render-style product never exposes to tenants (they never touch Kubernetes) — while workloads still share the host kernel and network, so NetworkPolicy + a sandboxed runtime are still required regardless. It is **not** a security boundary. Per-tenant control planes also multiply operational surface (upgrade/backup/monitor N apiservers) and carry always-on overhead that fights bex's dense bin-pack + free-tier-sleep economics. The isolation ladder is **namespace (default) → microVM (Kata/Firecracker/gVisor, hard)**; "tenant wants their own kubectl/CRDs" is a niche BYO-Kubernetes product feature, not an isolation strategy. (Rejected w1/m6, 2026-07-07.)

## Minimum bar for a meaningful milestone

Every proposed milestone must include:

- goal linkage (which project goal/pillar it advances),
- expected outcome (observable impact),
- why now (dependency/risk/sequence rationale),
- definition of done with testable end state.
