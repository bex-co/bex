# CLI sweep 980 — service events list and type vocabulary (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service. No fixture created. No new bug filed.

- `GET /services/{id}/events?limit=2` → timestamped events with opaque cursors (the baseline's recent hibernation events are visible).
- A bogus `type` filter → coded 400 that enumerates the entire accepted event vocabulary in the message — the refusal teaches the contract.
- Unknown service → 404.
