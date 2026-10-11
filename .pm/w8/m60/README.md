# w8 · m60 — Preserve accepted arguments when copying CLI commands

**Worker:** worker8 **Goal:** Copy from a working CLI log view produces a command that preserves its search and time filters. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Prepare an upstream command-aware Copy formatter | 50m | — |
| t002 | Verify shared Copy callers and command aliases | 25m | t001 |
| t003 | Adopt the accepted upstream correction through the CLI pin | 30m | t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 25m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On fresh owned Free fixtures, actual native Bex and adopted same-pin Render Copy preserve `--text 'LEFT RIGHT'` (one row) and `--text '"LEFT,RIGHT"'` (one literal term, one row). Replay the captured arguments unchanged; each copied result matches the original.
- A working `--text plain --start 30m` view copies an accepted relative time, with exit0 and its expected row. Absolute bounds preserve accepted nanoseconds. No Go struct/pointer representation appears; coordinate the independent082 precision fix.
- Plain and explicit two-OR-term controls retain their results; actual flag-parser checks cover counted shared callers, command aliases, enums, repeated arrays, positional spaces, Unicode, quotes and nil/default/accepted empty neighbors. Preserve bytes accepted by the existing parser, qualifying077's separate CRLF issue.
- The upstream correction is accepted, included in a reviewed Bex pin and released under separate authorization. A locally prepared patch cannot satisfy Done. Preserve existing server/UI filter semantics and the no-fork decision.
- Native evidence records version identities and gaps. Every fresh owned fixture reaches detail/list/runtime absence and quota convergence. Complete standing tasks and perform the final PM move only when all outcomes hold.

## Source + Goal linkage

- **Source:** ongoing user-requested functional `$qa-find-bugs-cli` loop, w8, sweep135, 2026-10-11. [Finding](finding.md), [ten native Copy captures](evidence/native-copy.json), [originals](evidence/controls.json), [replays](evidence/replays.json).
- **Goal linkage:** ADR008 dependable hosting, ADR010 precise logs and ADR018's imported Render CLI contract.
- **Expected outcome:** copied commands preserve already accepted search element boundaries and time values.
- **Why now:** released Bex and the unchanged exact upstream client reproduce all three failures against a healthy Free fixture; simple controls pass, and the same serializer remains in upstream main.
- **Render parity included:** a user-facing CLI formatter changes; verify the upstream contract and unchanged REST/GraphQL/MCP/dashboard behavior.
- **Scope:** one shared Copy serialization bug, minor severity, seven tasks /2h50m. Researched filing only; no implementation, upstream submission, release, commit or push in this hunt.

## Unverified

Live coverage is own web-service log phrase/literal-comma/relative-time Copy. Absolute time, Unicode/quote/CRLF neighbors and the parser are actual-pin local probes. Other command copies, datastore tails, unsupported workflows, deployed image revisions and direct Kubernetes teardown are unverified. Forty-two stack sites are a mechanical census, not42 proven affected/live callers. See the finding for setup/ingestion qualifications and completed cleanup.
