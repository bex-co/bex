# w9 · m174 — Session-scoped secrets for cloud agents (Devin parity)

**Worker:** worker9 **Goal:** a customer can give an agent session the API keys it needs (e.g. for integration tests or third-party APIs) without putting them in the repo or prompt, and they never land in the sandbox in plaintext beyond the session. **Status:** in progress — t001–t007 done; t008 closes after the live check on production (secret env var used by the agent, value absent from transcript/API reads) and the composer skeleton check at desktop and narrow-mobile widths

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Design note in ADR047/ADR094: session secrets model — **DONE** | 45m | — |
| t002 | Store and attach secrets (API) — **DONE** | 90m | t001 |
| t003 | Deliver into the sandbox and redact — **DONE** | 90m | t002 |
| t004 | Dashboard: add secrets when starting a session — **DONE** | 60m | t002 |
| t005 | Render parity (Devin reference for agent surfaces) — **DONE** | 25m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

## Definition of done

- A session given a secret can use it (live check: agent reads an env var and calls a test endpoint), the value never appears in transcript, logs or any API read, and Devin-equivalent inputs (stored refs + inline) exist on REST/GraphQL/MCP/dashboard.

## Source + Goal linkage

- **Source:** user request 2026-11-05 ("把 w21 以及需要跟进这些问题移动到 w9 然后你统一跟进解决", `/pm them all`), from the ADR094/ADR095 recap next-steps list.
- **Goal linkage:** ADR008 pillar 5 cloud agents; Devin parity (secrets).
- **Expected outcome:** agents can run real tasks that need credentials (integration tests, third-party APIs).
- **Why now:** top Devin gap by customer impact in the 2026-11-02 gap list.
- **Parity:** included — agent surface; Devin reference.
