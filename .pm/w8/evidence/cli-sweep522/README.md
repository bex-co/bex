# CLI sweep 522 — log time-window dialects (read-only)

**Verdict: no bug filed.** Log reads against a pre-existing baseline service (metadata reads only; nothing mutated, and the queries below return headers/counts in this record, not log contents). Same isolated session, HEAD build `218dffcd1`, 2026-10-19T09:26Z.

| probe | observed result |
| --- | --- |
| `--start notatime`, `--start=-5m` | Exit `1` client-side, naming the accepted forms (relative `1m`/`5h` or RFC3339). |
| `--direction sideways` | Exit `1` client-side, closed enum named. |
| Inverted window (start after end) | Exit `1`, server `400`: `startTime must be before or equal to endTime`. |
| `--start 999999999h` | Exit `1`, server `400`: `query range exceeds 720 hours` — the client's relative-to-absolute conversion is bounded server-side by name. |
| Fully future window (2030), ancient window (2020) | Exit `0`, honestly empty — no fabricated rows, no error. |
| `--start 5m` (relative), `--direction forward --limit 2` | Exit `0` with correctly bounded results. |

## Safety and unrun checks

- Read-only; the resource named is a pre-existing baseline service and only its log *metadata behavior* (counts/exit codes) is recorded here.
- Unrun: exactly-720h boundary (the range error names the cap; ±1h probing adds little), start==end instant window, timezone-offset RFC3339 forms.
- Deploy lag means deployed revisions can precede local `main`.
