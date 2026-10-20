# CLI sweep 1078 — disk reads on an empty workspace (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- The canary disk list is empty (`[]`) — no disks exist to probe against, and disk creation stays behind the paid gate (sweep 971).
- An unknown disk id hits the id-shape gate (`invalid path parameter "diskId"` 400) or, for the snapshots subresource, 404. Nothing leaks about disk internals.
