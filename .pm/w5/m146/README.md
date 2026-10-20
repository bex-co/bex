# w5 · m146 — Every request field Render declares on a served operation is handled or refused by name

**Worker:** worker5 **Goal:** a Render-shaped REST body on any served operation either works or answers a named refusal, never a generic `unknown field`, and a guard keeps it that way. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Guard: send every Render-declared request property through the composed server | 1h15m | — |
| t002 | Create accepts nested `serviceDetails.headers`, `routes` and `autoscaling` | 1h | t001 |
| t003 | Named refusals instead of `unknown field` | 45m | t001 |
| t004 | Ledger rows and composed-server tests | 20m | t002, t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Total: **8 tasks, about 5h 5m**.

## Definition of done

- A static site created with Render's `serviceDetails.routes` (SPA rewrite) returns 201 and serves the rewrite.
- `serviceDetails.autoscaling` on create sets autoscaling.
- The guard passes with no unexplained generic `unknown field`.

## Source + Goal linkage

- **Source:** third `/pm-brainstorm for w5` (2026-10-20), re-offered as the parity-only set and chosen by the user the same day. Of 208 Render operations, 152 are served, 54 deliberate non-goals and 1 owned; the remaining gaps are fields on served operations, confirmed against the composed server. Nothing tests Render-shaped bodies against the handlers (w8/136 only checked bex's own shape).
- **Goal linkage:** ADR006/ADR018 REST compatibility.
- **Expected outcome:** REST clients, agents and Terraform-style tools sending Render's documented body shape get Render's result or a named reason.
- **Why now:** this is the wrong-nesting family w1/m86, w6/m106 and w6/m130 each fixed one at a time; the guard closes the class.
- **Render parity:** included: REST create/PATCH.
