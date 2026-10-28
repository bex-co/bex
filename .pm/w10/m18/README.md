# w10 · m18 — Agent tool output: full output on demand, smaller inline transcript

**Worker:** worker10 **Goal:** The durable transcript keeps a short head and tail of each large tool output inline. The full output is stored separately and loaded only when the user clicks "Show full output". Long-session replays shrink further while the information stays available. **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est  | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | Store full tool outputs out of line (Postgres table, per-output cap) + lower inline cap   | 1.5h | —          |
| t002 | Fetch endpoint for one tool's full output (REST + GraphQL + MCP, session-scoped authz)    | 1h   | t001       |
| t003 | Dashboard "Show full output" on clipped tool steps                                        | 1h   | t002       |
| t004 | Verify on production: replay bytes of a fresh long turn and of `ags-dbf8nggkbcjs73a5r3u0` | 30m  | t003       |
| t005 | Render parity (REST/GraphQL/MCP full-output read; dashboard)                              | 20m  | t004       |
| t006 | Simplify                                                                                  | 20m  | t005       |
| t007 | Test coverage                                                                             | 30m  | t005       |
| t008 | Closeout                                                                                  | 10m  | t006, t007 |

## Definition of done

- Tool output above ~8,000 chars is stored inline as head+tail with the `[bex: N chars clipped]` marker. The complete text (capped at 1 MiB per output, with the cap marked) is stored in a separate table keyed by session, turn and toolCallId. Both the tee and the harvest write it, idempotently.
- A session member can fetch one tool's full output over REST, GraphQL and MCP. Other workspaces get not-found.
- The dashboard shows "Show full output" on clipped steps and loads it on demand.
- A fresh long turn's replay bytes are recorded in t004 and are lower than with the 30k cap. Existing transcripts keep working (old 30k-clipped items have no full copy; the UI says so).
- Backend real-DB suite, `make lint`, dashboard suite, Prettier green.

## Source + Goal linkage

- **Source:** promoted from `w10/004` (2026-10-27), filed by the w10/m16 DoD amendment. The long-session replay is 41% of the old bytes, and all of that is inline tool output.
- **Storage choice:** a Postgres side table, not object storage. The agent snapshot bucket was retired with hibernation (w10/m15), and outputs are bounded (1 MiB cap per output), so no new bucket or credential is needed. This is the sufficient option, recorded so it isn't a blocking decision.
- **Goal linkage:** ADR051 agent transcripts / ADR047 agent sessions (ADR008 AI-native pillar).
- **Expected outcome:** faster reopen of long sessions with no loss of tool output.
- **Why now:** it follows directly on m16, whose bytes target missed for exactly this reason.
- **Render parity included (t005):** a new read surface on REST/GraphQL/MCP plus the dashboard. There is no render.com equivalent.
