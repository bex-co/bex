# w1 · m24 — Multi-service `bex.yml`: Blueprint-shaped stack deploys

**Worker:** worker1 **Goal:** One `bex.yml` declares a whole stack — several services (web/worker/cron) plus managed databases — wired by `fromDatabase` env references, and one deploy call converges all of it, idempotently, with all-or-nothing validation. This is Render Blueprints' useful core (the spec, not a UI), and pillar 4 at stack scale: an agent's single action deploys the entire app. `DO_NOT_DO.md` routes exactly this here ("Render's only first-class DB→service linkage is the Blueprint `fromDatabase` field — if bex ever mirrors that, it's `bex.yml`/backend spec work (w1)") — spec work, never a dashboard picker. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `bex.yml` schema: `services:` list + `databases:`; validated against render.yaml vocabulary; legacy single-service files unchanged | 35m | —          |
| t002 | `fromDatabase` env references resolved via secretRef (never plaintext); `fromService` checked vs spec      | 30m | t001       |
| t003 | Deploy verb + `scripts/app-apply.sh` accept the stack form: DBs first, N App + M Database CRs, idempotent  | 35m | t001, t002 |
| t004 | Surfaces: MCP `deploy` stack form + validate-all-then-apply semantics; REST mapping documented             | 30m | t003       |
| t005 | Acceptance: web + worker + postgres in one `bex.yml` → all converge, web reads DB via injected env; re-deploy no-op | 25m | t004       |
| t006 | Render parity — bex.yml vs render.yaml Blueprint spec field-by-field; matrix Blueprint row updated         | 25m | t005       |
| t007 | Simplify — `/simplify` over what this milestone changed                                                    | 20m | t006       |
| t008 | Test coverage — schema validation, fromDatabase resolution, all-or-nothing + idempotency                   | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                             | 10m | t008       |

## Definition of done

A `bex.yml` declaring multiple services and a database deploys through one call (MCP `deploy` or the REST mapping): validation is all-or-nothing up front (one invalid entry rejects the whole apply with per-entry errors, nothing partially created), databases converge before dependents, `fromDatabase` injects working connection env without the value appearing in `bex.yml` or the App spec (secretRef), and repeating the deploy is an idempotent no-op. docs/ADR018-render-parity.md's Blueprint row records the field-level comparison.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12 (revival of the never-materialized 2026-07-09 w7-take-1 proposal); the `DO_NOT_DO.md` DB-linkage entry's explicit bex.yml/w1 pointer; docs/ADR018-render-parity.md "Blueprint / `render.yaml` IaC" row (◐ — consumes bex.yml single-service; no multi-resource form); render.yaml Blueprint spec.
- **Goal linkage:** pillar 4 (deploy-from-chat at stack scale — "deploy this repo: web + worker + postgres" as one agent call); pillar 1 (render.yaml compatibility is the artifact bex.yml exists to honor).
- **Expected outcome:** render.yaml-trained tooling and habits transfer to multi-service repos; the Blueprint matrix row's consume-side moves to ✅/◐-documented.
- **Why now:** every ingredient now exists to compose — service types (m15), env groups (m16), managed Postgres (m17) and Key Value (m14/w2-m7) — and each was built separately; the stack spec is the missing lid. More buildable today than when first proposed.
- **Render parity closing task: included** (the render.yaml field walk is the milestone's core); **no dashboard surface** — any Blueprint UI stays out per the DO_NOT_DO picker ban; a `/blueprints` REST *resource* (validate/list/sync) remains untracked-low, documented in t006 rather than built.
