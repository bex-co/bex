# w6 · m398 — A turn's transcript survives a sandbox that dies mid-turn

**Worker:** worker6 **Goal:** a session whose sandbox dies mid-turn keeps its conversation and commands up to about 30 seconds before the death, on every surface, so the failure can be investigated. **Status:** done — on production a session whose sandbox was killed mid-turn kept its transcript to 23 s before the kill, with no duplicates, truncated with the reason on REST, GraphQL, MCP and the session page (2026-11-17).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Driver: flush a running turn's completed transcript parts to bex-api — **DONE** | 2h | — |
| [t002](done/t002.md) | Completer: a failed turn keeps its flushed parts and says why the rest is missing — **DONE** | 1h | t001 |
| [t003](done/t003.md) | REST, GraphQL, MCP and the session page show a failed turn's saved transcript — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Production check: a QA session whose sandbox is killed mid-turn keeps its transcript — **DONE** | 45m | t003 |
| [t005](done/t005.md) | Render parity (Devin): a failed session's history reads alike everywhere — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 45m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

## Definition of done

On production, a session whose sandbox Pod is killed mid-turn shows its transcript up to about 30 s before the kill on REST, GraphQL, MCP and the session page, marked truncated with the reason. A later harvest of a turn never duplicates what was flushed.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` (2026-11-14), item 2. Found while investigating the 2026-11-13 `rt-flatten-abstractions` OOM (session `ags-dbrhfr0k58is73auo6a0`, 0 transcript rows after 371 model requests). Production: 9 failed sessions in 30 days with model spend kept no transcript.
- **Goal linkage:** ADR008 pillar 5 (hosted agent sessions). ADR047 consequence 5 promises a durable transcript store for terminal sessions; ADR051's harvest cannot run once the sandbox is gone.
- **Expected outcome:** a crashed or OOM-killed turn can be read and debugged.
- **Why now:** the routines run every three hours on 8 GiB sandboxes, and the next OOM's cause (w6/127 suspects `/tmp`) cannot be confirmed without its transcript.
- **Render parity:** included (Devin is the reference for agent sessions): the transcript surfaces change.

## Unblock 2026-11-16 (verified evidence)

- **Affected:** t004, t008. **Source:** verified evidence, no new user decision.
- **Finding:** the production deploy of `3c6654df6` (deploy.yml run 43816470794, started 06:23Z, succeeded 06:46Z) carries the running-turn harvest: `6ec9a62a5` is its ancestor, and Argo's `bex-operator` app has since synced main at `48bc2a0fe`. The time gate is clear.
- **Next:** t004 runs on production as the QA user (`bash scripts/qa-login.sh`). It starts a session with a long turn, deletes that session's sandbox Pod mid-turn, and reads the transcript on REST and the session page. Accept when the stored parts reach to about 30 s before the kill and the turn reads truncated with the reason. Then t008 closes the milestone. Devin is the reference: an errored session keeps the history it had.
- **Outcome of the run (2026-11-16):** the killed turn kept its harvested items and read truncated with the reason on REST, GraphQL and the session page. It lost the last 2 minutes, because the driver held parts in a 64 KiB buffer and the harvest ran once a minute (t004 has the evidence). Fixed in the same ship: the turn log writes buffered parts within 5 s, and the running harvest runs every 30 s. **Remaining gate (clears by itself):** a successful deploy.yml run whose head contains the `fix(agent-sessions): a running turn's parts reach the driver log within 5 s` commit. Then re-run t004, then t008.

## Unblock 2026-11-17 (verified evidence)

- **Affected:** t004, t008. **Source:** verified evidence, no new user decision.
- **Finding:** the deploy gate holds. deploy.yml run 44076479949 (head `db48cde2bae9`, 11:42–12:13Z) succeeded and wrote the production pin `fc4fbf28a` ("pin platform images to db48cde2bae9", 12:08Z). `14af9b20a` (the driver flushes buffered parts within 5 s; the Completer harvests every 30 s) is an ancestor of both. On the production app cluster, `bex-system/bex-api` runs `bex-operator@sha256:ced89914…` (2/2 updated) with `BEX_AGENT_SESSION_IMAGE` = `bex-agent-sandbox@sha256:cb82a446…`, the digests that pin wrote.
- **Next:** t004 re-runs the kill check as the QA user, as in its 2026-11-16 run. Accept when the last stored item ends within about 30–45 s of the kill and the turn reads truncated with the reason on REST, GraphQL, MCP and the session page. Then t008 closes the milestone. Devin is the reference: an errored session keeps the history it had.
- **Outcome of the run (2026-11-17):** session `ags-dbu92gogt2jc73djdvpg` was killed at 17:42:42Z. It kept its items up to 23 s before the kill, with none duplicated, and read truncated with the reason on REST, GraphQL and the session page. MCP's `get_agent_session_transcript` failed for any transcript with a part, because the SDK inferred a part's `json.RawMessage` as a byte array (t004 has the evidence). Fixed in the same ship. **Remaining gate (clears by itself):** a successful deploy.yml run whose head contains the `fix(mcp): get_agent_session_transcript returns a transcript's parts` commit. Then t004 reads the session over MCP and deletes it, and t008 closes.

## Unblock 2026-11-17 evening (verified evidence)

- **Affected:** t004, t008. **Source:** verified evidence, no new user decision.
- **Finding:** the deploy gate holds. deploy.yml run 44145201758 (head `e0a5ba20aac8`, 18:11–18:39Z) succeeded and wrote the production pin `e36296ae8` ("pin platform images to e0a5ba20aac8", 18:33Z). `bcdc75823` (`fix(mcp): get_agent_session_transcript returns a transcript's parts`) is an ancestor of `e0a5ba20aac8`. On the production app cluster, `bex-system/bex-api` runs `bex-operator@sha256:f7b06e9c…` (2/2 ready), the digest that pin wrote.
- **Next:** t004 reads `get_agent_session_transcript` for `ags-dbu92gogt2jc73djdvpg` over MCP as the QA user. Accept when it returns the 48 parts and turn 1 truncated with the reason. Then it deletes that session, and t008 closes the milestone.
- **Outcome of the run (2026-11-17 evening):** MCP `get_agent_session_transcript` returned the 48 parts (steps 1–23) and turn 1 `transcriptTruncated: true` with `sandbox terminated before completion`; the next page was empty. The session was then deleted over MCP (REST answers 404). The definition of done holds on all four surfaces; t004 and t008 done (t004 has the evidence).
