# CLI sweep 942 — list ordering is stable across identical reads (live, 2026-10-20)

Disposable fixture created for list presence and deleted (204, then GET 404). No new bug filed.

- Two identical `GET /services?limit=3` reads return the identical id sequence — the keyset order (sweep 859) does not shuffle between reads, so cursor walks cannot skip or repeat rows due to unstable ordering on an unchanged set.
