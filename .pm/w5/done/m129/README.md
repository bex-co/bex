# w5 · m129 — An unavailable Key Value says why, as a sentence and a code the dashboard translates

**Worker:** worker5 **Goal:** An unavailable Key Value reports why on REST, GraphQL and MCP: a fixed sentence plus a stable code. The dashboard shows that reason in the reader's language, as w5/079 did for Postgres. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The operator's Key Value Ready reasons are constants in `lego/types` — **DONE** | 45m | — |
| t002 | `KeyValueView` carries `statusReason` and `statusReasonCode` on every surface — **DONE** | 1h | t001 |
| t003 | The Key Value Details card shows the reason, translated by code — **DONE** | 1h | t002 |
| t004 | Render parity — **DONE** | 20m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 45m | t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- Every Ready=False reason the Key Value controller writes is a constant in `lego/types/v1alpha1/keyvalue_types.go`. The operator uses those constants, and no literal remains.
- An unavailable Key Value's REST, GraphQL and MCP reads carry `statusReason` and `statusReasonCode`. `statusReason` is a fixed sentence per reason, never the raw condition message. A healthy Key Value omits both: REST and MCP omit the keys, and GraphQL returns null.
- The dashboard's Key Value Details card shows the reason in en and zh, falling back to bex-api's sentence for a code it does not know. A vocabulary test keeps every reason bex-api explains translated.
- A cross-surface test pins both fields for a healthy and an unavailable Key Value on all three surfaces, and mutants confirm it.

## Source + Goal linkage

- **Source:** promoted from inbox `w5/101` (filed by w5/079's reuse review, 2026-10-06).
- **Goal linkage:**
  - ADR006: one view across REST, GraphQL and MCP.
  - ADR021 (Key Value management): an operator-managed datastore should explain its own state.
  - The dashboard's i18n rule: every user-visible string goes through `t()`.
- **Expected outcome:** an unavailable Key Value names its cause, such as a StatefulSet that failed to apply, a volume that could not be resized, or a backup job that could not be scheduled, in the reader's language. Today it reads only "unavailable".
- **Why now:** w5/079 built the exact pattern for Postgres, and the operator already records a reason on each of its failure paths. This applies the same shape while it is fresh.
- **Render parity included:** the change adds fields to the Key Value view on REST, GraphQL and MCP and a row to the dashboard.
