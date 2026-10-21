# w5 · m146 — Every request field Render declares on a served operation is handled or refused by name

**Worker:** worker5 **Goal:** a Render-shaped REST body on any served operation either works or answers a named refusal, never a generic `unknown field`, and a guard keeps it that way. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Guard: send every Render-declared request property through the composed server — **DONE** | 1h15m | — |
| t002 | Create accepts nested `serviceDetails.headers`, `routes` and `autoscaling` — **DONE** | 1h | t001 |
| t003 | Named refusals instead of `unknown field` — **DONE** | 45m | t001 |
| t004 | Ledger rows and composed-server tests — **DONE** | 20m | t002, t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

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

## Closeout (2026-10-20)

- `TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName` (`lego/backend/internal/api/render_declared_fields_test.go`) probes every declared property of create/update service (all five serviceDetails variants), Postgres and Key Value through the composed server. Zero are unprobed, and an unprobed property now fails the guard.
- A static site created with nested `serviceDetails.routes`/`headers` returns 201 and its App carries the rules (`TestRESTCreateStaticSiteTakesRoutesAndHeadersFromServiceDetails`). The static server already serves `spec.routes`.
- Nested `serviceDetails.autoscaling` on create sets the policy (`TestRESTCreateTakesAutoscalingFromServiceDetails`).
- `pullRequestPreviewsEnabled`, PATCH `serviceDetails.runtime` and PATCH `serviceDetails.cache` are refused by name (`TestRESTRefusesDeclaredButUnservedFieldsByName`). ADR018 has the rows.
- Render parity: REST is the only surface that takes Render's create body. GraphQL and MCP already take routes, headers and autoscaling through their own verbs, so there is no drift.
