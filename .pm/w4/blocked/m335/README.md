# w4 · m335 — An autoscaling change that only moves the CPU or memory target reads as a no-op in Events

**Worker:** worker4 **Goal:** an `autoscaling_config_changed` event says what changed on each side, targets included, in Render's `fromConfig`/`toConfig` shape, and a save that changes nothing records no event. **Status:** blocked — t001–t004 and t006–t008 done (shipped); t005 live replay waits for the next production deploy, then t009 closes

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Backend: record both sides' targets, and no event for an unchanged save — **DONE** | 30m | — |
| [t002](done/t002.md) | API: Render's `criteria` on REST and MCP, the targets on GraphQL — **DONE** | 20m | t001 |
| [t003](done/t003.md) | Dashboard: the Events line names each side's targets — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Docs: ADR018's event rows — **DONE** | 5m | t002 |
| [t005](t005.md) | Live verification on an owned autoscaled service | 15m | t003 |
| [t006](done/t006.md) | Render parity — **DONE** | 5m | t002 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t004 |
| [t008](done/t008.md) | Test coverage — **DONE** | 20m | t004 |
| [t009](t009.md) | Closeout | 5m | t005–t008 |

Nine tasks, 135m (2h15m).

## Source + Goal linkage

- **Source:** promoted from inbox note w4/590 (the w4/568 `/simplify` review, `/loopx w4` 2026-11-11) on 2026-11-11: the fix spans a migration, the audit recorder, the feed lookup, three API surfaces, the dashboard and a live replay.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); w4/568, w4/561 and w4/m328 made the neighbouring Events rows say what changed.
- **Render parity:** included. Render's `autoscaling_config_changed` details are its full autoscaling config on each side, `criteria` included (read-only GET on the reference account, 2026-11-11).
- **Follow-up:** w4/597. Render keeps a disabled config's values, and bex forgets them.

## Finding (from w4/590)

Why: since w4/568 the Events row shows an autoscaling change's before and after. A change to the target alone records the same range on both sides, so the owner sees a change that changed nothing.

- **Source / severity / estimate:** w4/568 `/simplify` review, `/loopx w4` 2026-11-11. **Minor**. ~45m.
- **Root cause:** `SetAutoscaling` (`lego/backend/internal/apps/service.go` near `:6228`) records `autoscalingMin/MaxFrom/To` only, and always writes a row, even when nothing changed.
- **Fix (target behavior):**
  - Record the CPU and memory targets on both sides (typed audit columns, the feed projection, REST, GraphQL and MCP).
  - The dashboard adds them to the line, e.g. "Autoscaling 1–3 instances, CPU 70% → 1–3 instances, CPU 60%".
  - Skip the row when nothing changed.
  - Check Render's `autoscaling_config_changed` details (`fromConfig`/`toConfig`) with a read-only GET on the reference account first, and match their shape.
- **Tests:** a target-only change records both targets; an unchanged save records no row.
- **Dedupe:** w4/568 (done) shows the range only.
