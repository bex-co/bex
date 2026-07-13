# w5 · m19 — Create wizard: env vars at create (+ GraphQL `createService(envVars:)`)

**Worker:** worker5 **Goal:** a service created from `/services/new` boots with its env config on the first deploy — no post-create Environment-tab round trip, no first-boot crash-loop — and GraphQL `createService` reaches envVars parity with REST/MCP create. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Live-capture Render's wizard env-var section (placement + affordances) → extend the capture doc | 30m | —          |
| t002 | Backend: `envVars` arg on GraphQL `createService` + settle the Environment-tab read-back semantics | 45m | —          |
| t003 | Wizard UI: env-var editor in the Settings step, wired into the create mutation           | 1h  | t001, t002 |
| t004 | i18n (en/zh) + validation and empty states                                               | 20m | t003       |
| t005 | Render parity — REST/GraphQL/MCP/UI consistency for create-time env vars vs render.com   | 30m | t004       |
| t006 | Simplify — `/simplify` over the code this milestone changed                              | 15m | t005       |
| t007 | Test coverage — meaningful tests for create-with-env behavior + failure modes            | 30m | t005       |
| t008 | Closeout — verify DoD met, then move the milestone to `done/`                            | 10m | t006, t007 |

## Definition of done

Creating a service in `/services/new` with env vars set results in the app running with those vars on its first deploy; GraphQL `createService(envVars:)` behaves identically to REST `POST /v1/services` `envVars` and the MCP create tools; the create-time-vars ↔ Environment-tab relationship is resolved and tested (the vars appear there post-create, or the chosen semantics are explicitly documented in ADR006/ADR018 with a follow-up filed — not silently divergent); `docs/render-artifacts/new-service-wizard.md`'s env-var row reflects a live capture, not the docs-fallback guess.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w5` 2026-07-12; the w5/m15 capture doc's explicit deferral (`docs/render-artifacts/new-service-wizard.md` — "Env vars inline | ✖ Not in v1"); `docs/ADR018-render-parity.md` "Create service" row (REST/MCP accept `envVars`, GraphQL doesn't); render.com/docs/web-services verified live 2026-07-12 ("Under the **Advanced** section, you can set environment variables and secrets…" — identical wording for all three source methods).
- **Goal linkage:** pillar 1 (Render-compatible surfaces; ADR006 "one core, thin adapters" — the GraphQL adapter currently lags REST/MCP on create).
- **Expected outcome:** a config-dependent service deploys working on the first attempt from the wizard (today it crash-loops until the user discovers the Environment tab); the `createService` surface asymmetry closes.
- **Why now:** w5/m15 + w5/m17 just completed the wizard's source and type coverage; this is the **last unblocked "Not in v1" deferral** in the capture doc (health-check-path is gated on `w1/m23/t001` → note `009`; region/runtime/build/start fields are deliberate omissions). Follows the w5/m9 / w5/m18 precedent of including the missing GraphQL half in the UI milestone.
- **Render parity closing task: included** — the milestone changes a GraphQL surface and the dashboard UI.
