# CLI sweep 815 — service instances surface (live, 2026-10-20)

Read-only probes; baseline only read. No new bug filed.

- `bex services instances srv-daif…` (baseline `hello-go`) → `[]`; REST `GET /services/{id}/instances` → 200 `[]` — the free baseline is asleep with no running instance to list, and both surfaces agree.
- Synthetic service id → CLI `Error: failed to list instances: 404 Not Found` (non-zero exit), matching the REST coded 404.
