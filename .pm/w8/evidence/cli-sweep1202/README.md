# CLI sweep 1202 — `bex logs` reads live service logs (live, 2026-10-20)

Read-only CLI sweep against the baseline service (never mutated). No new bug filed. Log message contents are not recorded here.

- `bex logs -r srv-daif6dsmg29s73d1umvg --limit 5 -o json` → exit 0 with structured entries carrying `type`/`resource`/`instance` labels for the baseline service: the CLI logs path reaches real runtime logs, not an empty stub.
- The same command with an unknown resource id → exit 1, `no resource found with ID or name …`: the CLI resolves the resource before querying and fails fast.
