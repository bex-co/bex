# CLI sweep 1149 — auth failures are uniform coded 401s (live, 2026-10-20)

Read-only sweep. No new bug filed. (A first pass without the CLI user-agent was blocked at the edge with 403/1010 for every variant — a harness artifact, separated and redone properly.)

- No `Authorization` header, a garbage bearer token, a bare `Bearer` with no token, and HTTP Basic credentials all → 401 with the same coded body (`unauthorized`). Nothing distinguishes a missing credential from a wrong one.
- A lowercase `bearer` scheme with an otherwise valid token → 401 as well: the scheme match is exact, which is what the pinned CLI always sends.
