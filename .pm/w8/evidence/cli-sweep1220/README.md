# CLI sweep 1220 — census clean at the 1220 mark (live, 2026-10-20)

Read-only census. No new bug filed.

- No `qa12*` fixtures survive in any family in `bex-canary`; baselines (service, Postgres, blueprint) all read 200, unmutated. Pin remains `94533d795067`; the port-below-1024 fix (`b01debc5d`) is still unpinned and on retest watch.
