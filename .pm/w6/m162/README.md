# w6 · m162 — Sandbox isolation fixtures come from the real session-egress generator

**Worker:** worker6 **Goal:** the ADR042 sandbox matrix's agent-session egress policy is rendered by `lego/backend/internal/sessionegress`, so it cannot drift from what bex-api creates. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Go helper that prints the real session-egress policy for a phase and allowlist](t001.md) | 45m | — |
| t002 | [`render_session_policy` calls the helper instead of hand-written YAML](t002.md) | 30m | t001 |
| t003 | [Test that the generated policy satisfies bex-api-session-egress](t003.md) | 30m | t001 |
| t004 | [Simplify](t004.md) | 15m | t002, t003 |
| t005 | [Test coverage](t005.md) | 20m | t004 |
| t006 | [Closeout](t006.md) | 10m | t005 |

## Definition of done

- A dispatched `isolation-matrix.yml` run passes its sandbox leg with the generated policy.
- No hand-written session-egress policy YAML remains in the script.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-06, proposal 4; the 2026-11-05 isolation-matrix fixes (`00170e192`, `d2b00c61e`).
- **Goal linkage:** ADR042/ADR047 sandbox isolation (Devin-equivalent sandbox boundaries): the matrix is a security guarantee only if it tests the real policy.
- **Expected outcome:** the sandbox matrix keeps testing bex-api's actual egress policy as it evolves.
- **Why now:** the leg has just started running for real; the drift went unseen for 2.5 months.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
