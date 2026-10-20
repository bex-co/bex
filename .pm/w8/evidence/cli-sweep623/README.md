# CLI sweep 623 — `logs --start`/`--end` dialects: client timestamp grammar, server window check

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Read-only against the baseline service; no fixture, no cleanup. Completes sweep 622's Unrun item.

- **Client-side grammar:** `not-a-time`, a bare date (`2026-10-19`), and a Unix-seconds string (`1234567890`) all fail in the flag parser: `invalid timestamp, time must either be relative (1m, 5h, etc) or in RFC3339 format`, exit 1, no request sent. The relative form (`--start 1h`) works.
- **Server window check:** `--start` after `--end` → `400 startTime must be before or equal to endTime`.
- **Empty windows are quiet successes:** a future start (`2999-…`) and a 2020 end both exit 0 with no rows — distinguishable from errors by exit code alone.
- **Narrow window is precise:** `--start 23:38:30Z --end 23:38:40Z` returned exactly the one in-window line (`23:38:36 GET /`).

## Unrun

- Relative end times and mixed relative/absolute windows.
- Timezone-offset RFC3339 forms (`+02:00`) against server-side window math.
