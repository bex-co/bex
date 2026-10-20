# CLI sweep 860 — service list filter discipline (live, 2026-10-20)

Read-only probes. No new bug filed.

- Malformed `createdAfter` → 400 `invalid query parameter "createdAfter"`; a future `createdAfter` → 200 `[]`; an inverted after/before window → 200 `[]` (empty intersection, not an error).
- Bogus `suspended=` → 400 enumerating `suspended|not_suspended`; bogus `type=` → 400 enumerating the five service types. Every filter refuses by name with its vocabulary.
