# CLI sweep 708 — CLI service-list filters (live, 2026-10-19)

Read-only.

- `bex services -e evm-0000000000000000000000 -o json` (synthetic environment) → `null` (empty result, same CLI nil-slice wart as sweeps 656/696).
- `bex services --include-previews -o json` → 8 services (flag accepted; no preview services exist to distinguish).
- `-e ''` → the unfiltered workspace list.

No bug filed.
