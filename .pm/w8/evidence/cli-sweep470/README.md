# pgcli session + log label discovery — sweep470

Passing functional coverage, no new issue. Same isolated session as sweeps 454–469, `bex-canary` (+ personal for Postgres), HEAD checkout build (`8333a44c5` tree), 2026-10-18T07:12–07:24Z.

`bex pgcli <dpg-id>` under an automated PTY completes the documented journey: launcher CA provisioning connects through `verify-full` with no hand-installed certificate, pgcli 4.6.0 reaches a real PostgreSQL 18.6 prompt, `SELECT 470 AS qa470pgcli` returns the row, and `\q` exits the child cleanly (`Goodbye!`). Two early harness attempts typed before the prompt was ready (first as timeout with empty capture); with readiness gating both defects disappeared — driver artifacts, recorded as such. Log label discovery honors the w6/m131 rule (discovery offers only what filters can match): after a single GET, `method` → `["GET"]`, `statusCode` → `["200"]`, app `level` → `["unknown"]`; an unknown label name refuses `400` listing the closed set.

Cleanup: exact-owned `srv-dba71lj6gvbc73encg6g` and `dpg-dba71l6qhqkc73a9rj30` deleted by ID (404 / named not-found verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; the loop continues from sweep471; m115 (shipped) / m116 + evidence 465–470 pending state unchanged except as noted per sweep.
