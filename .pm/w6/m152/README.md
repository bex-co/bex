# w6 · m152 — A disk owner sees when nightly snapshots are not running

**Worker:** worker6 **Goal:** the disk card says plainly when a disk's nightly snapshot could not be scheduled or is overdue, instead of showing an empty list that reads as "not yet" **Status:** todo (t001–t005 done; t006 awaits the production disk card)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Backend: `diskSnapshotHealth` from DiskReady + the last nightly window — **DONE** | 45m | — |
| t002 | Dashboard: a snapshot-health warning on the disk card, all locales — **DONE** | 45m | w6/m152/t001 |
| t003 | Render parity — **DONE** | 15m | w6/m152/t002 |
| t004 | Simplify — **DONE** | 15m | w6/m152/t003 |
| t005 | Test coverage — **DONE** | 20m | w6/m152/t003 |
| t006 | Closeout | 10m | w6/m152/t004, w6/m152/t005 |

## Definition of done

- GraphQL `diskSnapshotHealth(id)` returns `unavailable` when the App's `DiskReady` is False with a `Snapshot*` reason, and `overdue` when the snapshot of the first nightly window after the newer of the last snapshot and the disk's creation has not landed by 10:00 UTC. That is the window, the CronJob's 1h starting deadline and the 6h run deadline, shared through `lego/types` (`diskbackup.go`). Otherwise it returns `ok`. It is authorized like `diskSnapshots`.
- The dashboard disk card shows a warning for `unavailable` and `overdue`. It never shows the "no snapshots yet" message for an overdue disk. Every locale is translated.
- REST `GET /v1/disks/{id}/snapshots` is unchanged (Render shape).
- Backend and dashboard tests cover all three states and the window boundaries.

## Source + Goal linkage

- **Source:** [w6/094](../done/094.md), from [w6/089](../blocked/089.md). On 2026-10-22 production's bex-canary ClickHouse disk got no nightly snapshot, and nothing told its owner. The operator's `DiskReady` snapshot reasons have no reader in bex-api.
- **Goal linkage:** ADR082 managed disks, and backups an owner can trust. ADR008 says customer-facing behavior must be honest in every workspace.
- **Expected outcome:** an owner whose disk is unprotected sees it on the disk page within about 30 minutes of a missed window.
- **Why now:** a production disk is unprotected right now, invisibly. The cause investigation is gated (w6/088/089), but visibility is not.
- **Render parity:** included. REST stays Render-shaped. The health field is a bex GraphQL extension for the dashboard; t003 compares what Render shows for a failed snapshot.

## Implementation notes — 2026-10-22

- **Window:** the overdue cutoff is 10:00 UTC, not the first draft's 03:30. The simplify review found that a late-minute slot, the starting deadline or a long upload would raise false warnings.
- **Unavailable:** matches only the two scheduling reasons. `SnapshotStoreUnavailable` is a restore's reason. The verdict survives a failing store listing.
- **Shared code:** list and health use one gate (`authorizeDiskSnapshots`) and one listing (`diskSnapshotObjects`). GraphQL uses `IDVerb`/`TimeField`, and the card uses the shared `Alert` and `useLocalDateTime`.
- **Not changed:** the card still makes two GraphQL resolver calls (list and health), each listing the prefix once per mount, so a backend without the field cannot break the list.
- **Tests:** backend `disk_snapshot_health_test.go` covers window math, states, the reason set and listing failure. The dashboard card and hook tests pass. Totals: dashboard 5,508/5,508, operator disk tests, and lint in all three Go modules.
