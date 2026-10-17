# w4 · m228 — Preserve the log reader across display changes

**Worker:** worker4 **Goal:** wrapping and timestamp changes keep the current logical log line visible when its position remains reachable, while bottom-follow and virtualization retain their existing behavior. **Status:** todo

**Severity:** minor. **Estimate:** 140m / 2h20m across 7 tasks. Research and filing only; no product implementation.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Reconcile display geometry around a saved logical reader | 40m | — |
| [t002](t002.md) | Audit the three readers and preserve their existing controls | 20m | t001 |
| [t003](t003.md) | Replay desktop timestamps and mobile wrapping | 20m | t001, t002 |
| [t004](t004.md) | Render parity | 10m | t003 |
| [t005](t005.md) | Simplify | 10m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

## Definition of done

- Recreate the [captured owned Free image fixture](finding.md). Fresh desktop 1440×1000 → deploy detail → Maximize → hide timestamps → scroll to an older numbered row well before the end → show timestamps: the logical row remains visible through settling. The original ROW=130 was skipped to ROW=140. Keep the reader unpinned unless the user actually returns to the end.
- Fresh 390×844 → the same deploy → Maximize with wrapping/timestamps on → scroll to an older fully visible row → disable Wrap lines: the chosen row remains visible after remeasurement. The original ROW=128 was skipped to ROW=134. Restore a key with a bounded intra-row offset. A genuine near-end clamp is allowed when the range cannot put the row at the top; do not add blank padding or always jump to latest.
- Both transitions pass with stable rows under the locked React 19.3.1 / react-virtual 3.14.20 / virtual-core 3.19.5 source check and the deployed dashboard. CSS-only and measure-only shortcuts are not acceptance: retained local controls still lose the mobile reader.
- Preserve bottom-follow, scroll-up, Jump to latest, Exit full screen, nowrap horizontal scrolling, Unicode, current/older record search, pagination and stable identities. Keep the three-reader scope, bounded DOM, current cap and visible-window selection tradeoff. No backend cursor/transport/data change or new export feature.
- Meaningful regression checks and affected dashboard gates pass. Live replay deletes the owned service/deploy, verifies inventories/runtime/snapshots/public/API absence, revokes the exact session and stops local diagnostic servers. Only then finish and move tasks/milestone under the PM convention.

## Source + Goal linkage

- **Source:** authorized functional `$qa-find-bugs w4`, 2026-10-17, a896. [Finding and fix model](finding.md); [31 selected receipts, complete log responses, current-source diagnostics and cleanup](evidence/a896-log-display-position.json).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md)'s usable hosting diagnostics and [ADR010](../../../docs/ADR010-observability.md)'s existing log reader.
- **Expected outcome:** a display preference does not skip the line being investigated; geometry reconciles around a logical reader rather than stale pixels.
- **Why now:** fresh desktop/mobile controls reproduce different skipped lines. The unchanged current-source renderer with locked dependencies reproduces both with a constant array, excluding polling/new records and an already-resolving dependency upgrade as the sole explanation.
- **Standing tasks:** Render parity covers the tenant-facing controls, followed by Simplify, Test coverage and Closeout. No authenticated Render scroll policy is claimed. Preserve [w9/m83](../../w9/done/m83/README.md), [w4/m151](../done/m151/README.md) and [w4/m192](../done/m192/README.md).
