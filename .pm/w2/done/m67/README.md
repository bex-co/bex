# w2 · m67 — ADR059 Active tier: last-interaction idle grace + per-workspace live-sandbox cap

**Worker:** worker2 **Goal:** the Active tier of ADR059's state machine lands as a real subset of the final model — a finished session's sandbox stays alive through a bounded, last-interaction-measured idle grace (so Open in Zed's first connect is reliable), and a per-workspace concurrent live-sandbox cap bounds the cost — replacing the current "reap ~15s after the turn, unless an SSH session is open right now" behavior. **Status:** done (2026-08-15). Shipped in `7bf7dcde`; rolled to prod via the `81760b48` deploy (pin `12d1b730`). Idle grace (`BEX_AGENT_SANDBOX_IDLE_TTL`, default 30m; `idle = now − max(last turn end, last SSH disconnect)`; open editor pins at zero; `0` ⇒ ADR054 D6 immediate reap) generalizes the m65 teardown deferral, and the per-workspace live cap (`BEX_AGENT_MAX_LIVE_SANDBOXES_PER_WORKSPACE`, default 5; `AGENT_SESSION_LIVE_LIMIT` 409 across REST/GraphQL/MCP) bounds cost. t003 simplify: reviewed — cap check and idle clock are already helper-factored, single reaper seam kept for m68's hibernate swap; nothing behavior-preserving to change. Full backend suite green; idle math + cap semantics unit-tested on a fake clock.

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Last-interaction idle model + grace reaper (`BEX_AGENT_SANDBOX_IDLE_TTL`)                  | 45m | —          | — **DONE** |
| t002 | Per-workspace concurrent live-sandbox cap                                                  | 40m | t001       | — **DONE** |
| t003 | Simplify pass over the milestone's changes                                                 | 30m | t002       | — **DONE** |
| t004 | Test coverage for idle math, connected-never-expires, and cap behavior                     | 40m | t002       | — **DONE** |
| t005 | Closeout                                                                                   | 15m | t004       | — **DONE** |

## Definition of done

With the operator defaults: a session that finishes a turn keeps its live sandbox until `idle > BEX_AGENT_SANDBOX_IDLE_TTL`, where `idle = now − max(last turn end, last SSH disconnect)` — it **never counts down while an SSH/editor session is connected** (ADR054 D6 behavior preserved as a special case), and a reconnect or new turn resets the clock. A workspace attempting to exceed its concurrent live-sandbox cap gets a named refusal, not a silent queue. Explicit Cancel still reclaims immediately. Backend suite green; the reaper's decisions are unit-tested against a fake clock, no sleeps.

## Source + Goal linkage

- **Source:** [docs/ADR059-agent-sandbox-hibernation.md](../../../docs/ADR059-agent-sandbox-hibernation.md) (Proposed 2026-08-14) — the Active tier named in D7 as the first increment; extends w2/m65's ADR054 D6 teardown deferral into the full idle model.
- **Goal linkage:** pillar 5 — makes Open in Zed (w2/m65) reliably connectable (today the sandbox is reaped ~15s after a turn unless the user already connected), while keeping live-pod cost bounded.
- **Expected outcome:** a user can finish a turn, click Open in Zed within the grace window, and connect; idle sandboxes stop accumulating (TTL) and no workspace can exhaust the `bex-sandbox` pool (cap).
- **Why now:** m65's live acceptance proved the timing gap — the author raced the Completer to connect. This is the smallest shippable slice of the decided ADR059 model, is NOT throwaway (it is the Active state of D2's machine), and unblocks daily use while the m68 spike/hibernate work proceeds.
- **Render parity task OMITTED:** lifecycle-internal only — no REST/GraphQL/MCP field or dashboard UI changes (the existing `sshAddress` phase-gating already reflects sandbox liveness). If a cap-refusal error code surfaces on the create/steer path, it reuses the existing error envelope; note any new code in ADR006's error table during implementation.

## Notes

- The eventual Hibernated tier (m68) replaces "reap" with "hibernate" at the same decision point — build the reaper so the terminal action is a seam, not an inline Terminate.
