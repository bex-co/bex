# w4 · m228 — Preserve the log reader across display changes

**Worker:** worker4 **Goal:** wrapping and timestamp changes keep the current logical log line visible when its position remains reachable, while bottom-follow and virtualization retain their existing behavior. **Status:** blocked — t001, t002, t004–t006 done 2026-10-17; t003 (desktop timestamps / mobile wrapping replay on a deployed build) and t007 closeout open

**Severity:** minor. **Estimate:** 140m / 2h20m across 7 tasks. Implemented 2026-10-17; awaiting the deployed-build replay (t003).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Reconcile display geometry around a saved logical reader — **DONE** | 40m | — |
| [t002](done/t002.md) | Audit the three readers and preserve their existing controls — **DONE** | 20m | t001 |
| [t003](t003.md) | Replay desktop timestamps and mobile wrapping | 20m | t001, t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 10m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 10m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

## Definition of done

- Recreate the [captured owned Free image fixture](finding.md). Fresh desktop 1440×1000 → deploy detail → Maximize → hide timestamps → scroll to an older numbered row well before the end → show timestamps: the logical row remains visible through settling. The original ROW=130 was skipped to ROW=140. Keep the reader unpinned unless the user actually returns to the end.
- Fresh 390×844 → the same deploy → Maximize with wrapping/timestamps on → scroll to an older fully visible row → disable Wrap lines: the chosen row remains visible after remeasurement. The original ROW=128 was skipped to ROW=134. Restore a key with a bounded intra-row offset. A genuine near-end clamp is allowed when the range cannot put the row at the top; do not add blank padding or always jump to latest.
- Both transitions pass with stable rows under the locked React 19.3.1 / react-virtual 3.14.20 / virtual-core 3.19.5 source check and the deployed dashboard. CSS-only and measure-only shortcuts are not acceptance: retained local controls still lose the mobile reader.
- Preserve bottom-follow, scroll-up, Jump to latest, Exit full screen, nowrap horizontal scrolling, Unicode, current/older record search, pagination and stable identities. Keep the three-reader scope, bounded DOM, current cap and visible-window selection tradeoff. No backend cursor/transport/data change or new export feature.
- Meaningful regression checks and affected dashboard gates pass. Live replay deletes the owned service/deploy, verifies inventories/runtime/snapshots/public/API absence, revokes the exact session and stops local diagnostic servers. Only then finish and move tasks/milestone under the PM convention.

## Source + Goal linkage

- **Source:** authorized functional `$qa-find-bugs w4`, 2026-10-17, a896. [Finding and fix model](finding.md); [31 selected receipts, complete log responses, current-source diagnostics and cleanup](evidence/a896-log-display-position.json).
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md)'s usable hosting diagnostics and [ADR010](../../../../docs/ADR010-observability.md)'s existing log reader.
- **Expected outcome:** a display preference does not skip the line being investigated; geometry reconciles around a logical reader rather than stale pixels.
- **Why now:** fresh desktop/mobile controls reproduce different skipped lines. The unchanged current-source renderer with locked dependencies reproduces both with a constant array, excluding polling/new records and an already-resolving dependency upgrade as the sole explanation.
- **Standing tasks:** Render parity covers the tenant-facing controls, followed by Simplify, Test coverage and Closeout. No authenticated Render scroll policy is claimed. Preserve [w9/m83](../../../w9/done/m83/README.md), [w4/m151](../../done/m151/README.md) and [w4/m192](../../done/m192/README.md).

## Implementation record (2026-10-17)

- **t001:** `LogLineList` tracks its display mode (`wrap`, `showTimestamps`). On a change it saves a settling target: the reader's top-row key and intra-row offset, or the end when pinned. It turns off the virtualizer's size-change scroll compensation (`shouldAdjustScrollPositionOnItemSizeChange`), drops old-mode sizes (`measure()`) and explicitly remeasures the mounted rows, since a row whose height did not change gets no ResizeObserver report. Then, on every layout pass while settling, it re-places the target row by key with the offset clamped to the row's new height (`placeRow`, now shared with the existing prepend/expiry anchoring). Settling ends after 250 ms without a pass, at most 1 s after the toggle (so a busy live tail cannot extend it), on reader input (wheel, touch, pointer, key) or on Jump to latest. Scroll events during settling neither replace the target nor change the pin, and at the end the anchor and pin are recomputed from the final geometry. The viewport opts out of native overflow anchoring (`overflow-anchor: none`), since the list anchors itself. Pinned readers keep following the end; a near-end row clamps to the reachable range without padding. Stable keys, the cap, bounded DOM and every other reader control are unchanged.
- **t002 census:** the three production readers share this one component: service LogViewer and DatastoreLogViewer (default flags, so no toggle) and DeployLogPanel (both toggles), mounted once on the deploy detail page for the service and static routes. Postgres/Key Value use the datastore reader.
- **t004:** ADR010 records the display-change behavior; no API change.
- **t005:** three /simplify passes (shared `placeRow`/`recordAnchor`, settle cap, Jump ends settling, re-anchor at the end, override only when there is a target, explicit remeasure of mounted rows).
- **t006:** `log-line-list-display-reflow.test.tsx` uses a triggerable ResizeObserver that reports only rows whose height changed, and fires the scroll event a programmatic `scrollTop` change produces. Wrapped and one-line heights (40/20px) both differ from the 24px estimate, and the reader's row is read from the rendered DOM. It covers wrap on keeping the row and the unpinned state, a timestamp toggle whose rows keep their height, a clamp into a shorter row that holds after a later append, settling ending within its cap under a busy live tail, and a pinned reader. The first three fail against the old component and the busy-tail case without the cap. The shared geometry helper gained an optional per-row height. Dashboard suite (5184 tests), lint and typecheck pass. The real-browser behavior (native anchoring, actual ResizeObserver timing) is t003's deployed replay.
