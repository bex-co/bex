# w5 · m128 — Log, forbidden and protected-environment refusals carry codes, and the dashboard branches on them

**Worker:** worker5 **Goal:** Every refusal the dashboard branches on carries a stable code on REST, GraphQL and MCP, so rewording a bex-api message can no longer change what the dashboard shows. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Inventory the four refusals and their dashboard markers, and choose the codes — **DONE** | 30m | — |
| t002 | Code the log-source refusals on every surface — **DONE** | 45m | t001 |
| t003 | Code forbidden on every surface — **DONE** | 45m | t001 |
| t004 | Code the protected-environment confirmation on every surface — **DONE** | 45m | t001 |
| t005 | Dashboard branches on the codes; the text markers go — **DONE** | 1h | t002, t003, t004 |
| t006 | Render parity — **DONE** | 20m | t005 |
| t007 | Simplify — **DONE** | 15m | t006 |
| t008 | Test coverage — **DONE** | 45m | t007 |
| t009 | Closeout — **DONE** | 10m | t008 |

## Definition of done

- Each of these refusals carries one stable code on GraphQL `extensions.code`, the REST error body and the MCP error text: the pod-log source missing, the durable log store missing, forbidden, and a protected environment's missing confirmation.
- Their messages are unchanged, since Render CLI users read them.
- The dashboard decides each of the four cases by code: `use-log-history`, `use-deploy-logs`, `graphql-series`, `datastore-log-viewer`, `isForbiddenError` and `protected-confirmation`. No substring of server copy remains in those decisions.
- Tests pin the codes per surface, and the dashboard's decisions by code. A message reworded in a test changes no dashboard outcome.

## Source + Goal linkage

- **Source:** promoted from inbox w5/097, found by w5/m125 t001–t002 (2026-10-06).
  - w5/m125 moved Blueprint takeovers onto `errorDetails{code}`.
  - Four dashboard decisions still substring-match server copy: `"durable log store"`, `"logs source not configured"`, `"forbidden"` and `"protected environment"`.
- **Goal linkage:**
  - ADR006: one error dialect across REST, GraphQL and MCP.
  - The dashboard decides by code, as w5/m125 and w5/m118 established.
- **Expected outcome:** bex-api can reword these messages without changing the dashboard's behaviour, and API clients can branch on the codes.
- **Why now:** w5/m125 left exactly these four markers, and the backend sentinels involved are few and well-defined.
- **Render parity included:** the change touches the REST, GraphQL and MCP error envelopes.
