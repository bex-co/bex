# w4 · m209 — Keep metrics geometry stable and labels readable

**Worker:** worker4 **Goal:** metrics preserve their populated chart layout through loading and show complete axis labels **Status:** todo

**Estimate:** 4h across eight tasks. [Finding, scope and prior guarantee audit](finding.md) · [Durable live evidence](../evidence/m209-keyvalue-metrics-geometry.json).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Share the responsive chart viewport across data and pending states | 45m | — |
| [t002](t002.md) | Match the Key Value Metrics route preview to its real panel | 35m | t001 |
| [t003](t003.md) | Reserve enough axis gutter for complete formatted ticks | 45m | — |
| [t004](t004.md) | Verify the shared chart consumers and caller extras | 30m | t002, t003 |
| [t005](t005.md) | Render parity | 15m | t004 |
| [t006](t006.md) | Simplify | 10m | t005 |
| [t007](t007.md) | Test coverage | 50m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

## Definition of done

- On an owned private Free Key Value with populated Disk, Memory and Connections, cold native home → resource link → Metrics, hold the unchanged successful KeyValue response seven seconds at1440×1000 and390×844. The pending card retains the actual heading/description and three chart sections; it contains no phantom filter row. Compare outer bounds, padding and section positions against populated ready. At the recorded widths ready cards are998.5625/514.390625px, headers40px, plot widths854/316px; allow ordinary pixel rounding.
- Repeat with the unchanged complete DISK/DISK_CAPACITY/MEMORY/CONNECTIONS response batch held seven seconds after resource resolution. Pending plots retain the populated600:180 ratio:256.1875/94.796875px in this fixture. The three chart sections and footer of the card stay at their ready positions; capacity arrival preserves heading-row height. Keep already-populated charts visible during a poll.
- Fresh-load the recorded populated memory axis case at both widths. Full536.7 KiB text is readable, and its actual SVG bounding box starts at least4 viewBox units inside the canvas. Short26.0 KiB control remains readable. Values/formatting are unchanged and no text is moved outside the card. Keep plot, tooltip and marker x geometry consistent with its gutter.
- Complete t004's separately marked browser verification of empty/error/unavailable/throttle, long tick variants, shared line/bar/caller extras and untouched route producers before claiming those source-traced cases fixed. Do not infer success from these KV screenshots.
- Complete meaningful behavioral browser geometry coverage and required affected dashboard checks. Delete every exact owned fixture, restore full baseline ID sets, remove own observers/temporary handles and revoke the own session. Only then close.

## Source + Goal linkage

- **Source:** continuous functional $qa-find-bugs w4, 2026-10-14 a470 using the muse.env helper. Two minor findings have separate causes and implementation tasks; requests, full responses, measured DOM and exact cleanup are durable.
- **Goal linkage:** ADR008's dependable hosting dashboard and operational observability; existing metrics contract in ADR006/ADR018 and dashboard/AGENTS.md's mandatory pending/ready shape rule.
- **Expected outcome:** opening Metrics does not expand or collapse the chart stack as data arrives, and ordinary byte measurements remain readable.
- **Why now:** actual desktop/mobile failures remain despite the completed m63/m79 loading guarantees; these shared consumers make a screenshot-only local workaround incomplete.
- **Scope:** shared chart presentation plus the one KV route producer, its full consumer audit and behavioral verification. Keep APIs, units, values, permissions, telemetry, plan gates, query/poll policies and unrelated datastore/service skeleton regions unchanged. Tick precision/i18n issues remain with w4/296/297.
- **Render parity:** included because visible UI changes. Compare [Render Service Metrics](https://render.com/docs/service-metrics), preserve Bex's documented metric-set choices and record any drift. Hosted Render loading dimensions are unverified.
- **Observed versus unverified:** only an owned Free Valkey8 fixture was driven live. Postgres, paid service disks, application/network/scaling/bar charts, event strips, multiseries, errors and unavailable states are t004 verification work. No paid resource is needed for the reproduced DoD.
