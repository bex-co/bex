# w4 · m85 — Role-aware write controls across remaining dashboard editors

**Worker:** worker4 **Goal:** make the remaining `can_create`-gated dashboard controls honest before interaction, so contributors see a disabled control with a reason instead of an editable flow that predictably ends in a 403. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Gate service and Environment Group env-var writes — **DONE** | 45m | — |
| t002 | Gate Build Filters through the viewer-capabilities projection — **DONE** | 30m | — |
| t003 | Gate Postgres and Key Value destructive deletion controls — **DONE** | 45m | — |
| t004 | Render parity — **DONE** | 30m | t001, t002, t003 |
| t005 | Simplify — **DONE** | 20m | t004 |
| t006 | Test coverage — **DONE** | 45m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

## Definition of done

- A contributor lacking `can_create` sees service env-var add/edit/delete, Environment Group env-var writes, Build Filters, and Postgres/Key Value destructive deletion controls disabled with an intelligible permission reason. Postgres/Key Value plan changes and suspend/resume/restart stay enabled because their shared Core verbs require `can_operate`, which contributors hold.
- Disabled controls cannot open a save or confirmation flow and cannot dispatch the protected mutation; authorized users retain byte-identical behavior.
- Every decision comes from the existing viewer-capabilities projection and shared permission UI primitives. The dashboard does not infer roles or duplicate server authorization policy.
- Role-split component tests cover contributor and authorized states for each control family, including an assertion that disabled interactions issue no mutation.
- ADR018's Environment variables, Build Filters, managed Postgres lifecycle, and managed Key Value lifecycle rows remain truthful across REST, GraphQL, MCP, and UI.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w4`, 2026-08-18, promoting the bounded remainder recorded in `.pm/w9/048.md` after `w9/m84` established the viewer-capabilities and disable-with-reason pattern.
- **Goal linkage:** `.pm/GOAL.md` goals 5 and 7 plus ADR008's Render-compatible, multi-tenant control-plane boundary: authorization must be both enforced by the server and represented honestly by its first-party client.
- **Expected outcome:** contributors no longer discover a known permission denial only after editing a value and pressing Save; administrators and other authorized users see no behavior change.
- **Why now:** `w9/m84` just shipped the reusable capability hook, tooltip, and disabled-reason conventions. The adjacent controls are a finite follow-up over the same authorization relation, so completing them now avoids leaving two conflicting dashboard permission idioms.
- **Render parity:** included because this changes tenant-facing UI behavior on capability rows ADR018 already marks shipped; the pass must verify the UI remains consistent with the shared REST/GraphQL/MCP authorization semantics and record any Render role-behavior drift.
