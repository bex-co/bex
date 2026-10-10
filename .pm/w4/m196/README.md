# w4 · m196 — Keep plan-picker loading geometry faithful

**Worker:** worker4 **Goal:** the Instance Type page preserves its ready Free/Paid structure and footer position while catalog data load **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Match the known-type picker loading groups and dimensions | 40m | — |
| t002 | Audit and verify the shared plan-loading callers and types | 30m | t001 |
| t003 | Render parity | 15m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 30m | t003 |
| t006 | Closeout | 5m | t004, t005 |

## Definition of done

- Fresh-load an owned Free Web service's Plan page with the unchanged API response containing InstanceTypes held for four seconds at 1440×1000. Pending and ready preserve the separate Free/Paid regions, responsive columns, card dimensions and action slot; the recorded 355→551-pixel card expansion is gone. Free is checked after settle and Save Changes stays disabled.
- Repeat at 390×844: pending and ready preserve the same grouped card geometry instead of the recorded 771→1015-pixel expansion; document width remains 390. Compare within-card dimensions separately from bootstrap header movement.
- The deciding data do not enable Save or selectable tier controls while unresolved; once ready, the existing current-plan selection is preserved. No plan mutation is needed to verify this shape.
- Complete the separate t002 audit and record the three shared callers' known/unknown type handling and sibling results. Those initial/route-pending and non-Web branches are unverified live in the source finding; do not report them restored based solely on this Web catalog replay.

## Source + Goal linkage

- **Source:** functional $qa-find-bugs w4, 2026-10-10 pass a77, muse.env login helper. [Finding and durable replay](finding.md). Severity minor; 196-pixel desktop and 244-pixel mobile within-card expansion.
- **Goal linkage:** dependable hosting configuration UX in [ADR008](../../../docs/ADR008-vision.md), with the existing plan surface in [ADR018](../../../docs/ADR018-render-parity.md) and dashboard/AGENTS.md's mandatory loading shape.
- **Expected outcome:** opening Instance Type does not rearrange its Free/Paid groups and action slot as the response arrives.
- **Why now:** a reproducible gap remains in the existing w5/m79 completion guarantee. It predates that milestone in source; no later regression or undeployed fix was demonstrated.
- **Scope/size:** about 2h10 across implementation, the shared-caller/type audit and meaningful geometry verification. This includes unresolved-type handling rather than patching one screenshot.
- **Render parity:** included for the visible UI change; use the existing captured ready grouping, preserve deliberate price/contact omissions and API semantics. No Render loading-animation parity was observed.
- **Cleanup:** owned fixture deleted, six baseline ID families restored, own session revoked, old whoami 401 and cookies 0. Local ledger cleanupVerified true. Evidence screenshots exist and were viewed.
