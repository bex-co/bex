# w1 · m87 — Persistent disks: make the running system match its records

**Worker:** worker1 **Goal:** every claim ADR018 and ADR082 make about persistent disks becomes true of production, not just of the code. **Status:** todo (t005, t006 done; t001-t004 blocked on infra credentials)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Provision the disk-snapshot bucket + a disk-dedicated age keypair | 45m | — |
| t002 | Wire `BEX_DISK_SNAPSHOT_*` into the operator and bex-api manifests | 45m | t001 |
| t003 | Mirror the new variable names into `.env.example` | 15m | t002 |
| t004 | Verify a real snapshot end to end on a live disk | 1h | t002 |
| t005 | Say "not configured" instead of "internal error" on the Snapshots card | 30m | — | — **DONE**
| t006 | Verify the disk usage series against real Prometheus | 45m | — | — **DONE**
| t007 | Render parity check | 30m | t004, t005, t006 |
| t008 | Simplify pass | 30m | t007 |
| t009 | Test coverage | 45m | t007 |
| t010 | Closeout | 30m | t009 |

## Progress notes

**t006 answered, no code change needed (2026-08-24).** The "No data in range" chart was kubelet volume-stats warm-up on a freshly attached disk, not a broken matcher — the task named both possibilities and required evidence rather than an assumption. Queried the live API for the same metric on a database disk and the service disk: both return real series, and the service series carries `instance=disk-tea-da2isimlm39c739m4ofg-beancount-ledger`, i.e. exactly the claim `appv1alpha1.DiskPVCName` derives. The chart now renders. The `kind="service"` path is correct.

That diagnostic surfaced a **pricing defect filed as [078](../078.md)**: the live 1 GB disk sits on a ~10 GiB volume because Hetzner's volume minimum is 10 GB — which ADR082's own context section records — while `diskMinSizeGB = 1` and the UI ships 1 GB and 5 GB chips. bex charges $0.175 and pays ~$0.50 for a 1 GB disk. It needs a product decision, so it is a separate note rather than folded in here.

## Definition of done

A disk-bearing service on production has a backup CronJob; at least one real snapshot object exists under its prefix; `GET /v1/disks/{diskId}/snapshots` lists it; a restore on a scratch disk returns the pre-snapshot bytes and discards the post-snapshot ones; deleting that disk purges its objects. The Snapshots card never renders "internal error" for an unconfigured store, and a genuine failure stays visibly distinct from "never configured". The Disk usage chart either shows a real series or its absence is explained by a Prometheus query proving the series does not exist yet. `.env.example` carries every new name.

## Source + Goal linkage

- **Source:** [.pm/w1/077.md](../done/077.md) (snapshots ship inert) plus two defects found in the 2026-08-24 production walkthrough of the w1/m86 Disk tab, neither of which had a note.
- **Goal linkage:** ADR018 ledger truthfulness and ADR082 D5. The disk row now claims ✅ on all four surfaces and advertises daily snapshots with seven-day retention; that is currently true of the code and false of the running system. A parity ledger that overstates is worse than one that admits a gap.
- **Expected outcome:** a tenant who attaches a disk actually gets backups, and the UI tells the truth when they are unavailable.
- **Why now:** m83–m86 shipped the whole disk feature; this is the gap between "shipped" and "working in production", and it is cheapest to close while the design is fresh. The two UI defects were found by driving the real page — the unit suites were green throughout, which is exactly why they need regression tests.
- **Render parity task included** (t007): the milestone changes tenant-facing snapshot availability, its error states, and the usage chart.
