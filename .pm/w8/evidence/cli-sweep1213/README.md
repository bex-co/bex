# CLI sweep 1213 — CLI jobs surface carries the coded 410 (live, 2026-10-20)

Baseline service used read-only; the create attempt was refused, so nothing was created or mutated. No new bug filed.

- `bex jobs list <baseline> -o json` → exit 0 `[]`; on an unknown service → exit 1 with the coded 404.
- `bex jobs create <baseline> --start-command …` → exit 1 surfacing the API's `410 (ONE_OFF_JOBS_UNSUPPORTED)` with its full guidance message — the documented divergence (sweep 1145) reaches CLI users verbatim, with the supported alternatives named.
