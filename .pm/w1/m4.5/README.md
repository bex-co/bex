# w1 · m4.5 — Sleep in the dashboard: hibernated state, idle-timeout setting, wake UX

**Worker:** worker1 **Goal:** The human-facing half of m4's `sleep = free`: the dashboard distinguishes an auto-hibernated app from a manually suspended one, explains "wakes on the next request", lets the owner set the idle timeout (`spec.idleTTLSeconds`) from service Settings, and shows the wake transition live. **Render-informed, not strict parity:** the wake/cold-start messaging mirrors Render's free-instance banner; the Sleeping badge and the configurable idle timeout are **deliberate bex extensions** (Render keeps spun-down services showing as live and its ~15-minute window is fixed) — recorded as divergences, not parity. **Status:** todo (unblocked — m4 shipped 2026-07-08; its tasks live in `../done/m4/`)

## Tasks (in order)

| id   | title                                                                              | est | depends_on           |
| ---- | ----------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Capture Render's free-instance sleep UX + map bex states (Hibernated vs Suspended)  | 25m | —                    |
| t002 | Surface the sleeping state: list + overview badge, "wakes on next request" hint     | 30m | t001, w1/m4/t005     |
| t003 | Idle-timeout control in Settings wired to `spec.idleTTLSeconds` (incl. API gap)     | 35m | t001, w1/m4/t005     |
| t004 | Wake UX + acceptance: sleeping app wakes on request; dashboard reflects the flip    | 25m | t002, t003           |
| t005 | Simplify — `/simplify` over the code this milestone changed                          | 20m | t004                 |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped             | 30m | t004                 |

## Definition of done

A free-tier app idle past its TTL shows as **Sleeping** (not Suspended) in the services list and overview, with the wake-on-request explanation; changing the idle timeout in Settings persists to `spec.idleTTLSeconds` and the operator honors it; hitting the app's URL wakes it and the dashboard transitions Sleeping → Running without a manual refresh beyond the existing polling. Manual suspend/resume rendering is unchanged.

## Source + Goal linkage

- **Source:** user request 2026-07-08 (`/pm arrange w1 m4.5 for frontend work of m4`); pattern precedent: w4/m6.5 (dashboard half of m6), w5/m7 (Settings + capture-first).
- **Goal linkage:** vision "Free tier = sleep" economics (m4) made visible and manageable by humans. Render-informed (banner copy = parity; Sleeping badge + idle-timeout knob = bex extensions under the "safe superset" rule, since sleep is a first-class observable phase in bex's model).
- **Expected outcome:** users can see why their free app is not running, control its idle window, and trust the wake path — no support-style confusion between suspended and sleeping.
- **Why now:** paired with m4 so the UX ships with the mechanism instead of trailing it (the w4/m6→m6.5 pattern); the capture task is prep-parallel while m4 is built.
