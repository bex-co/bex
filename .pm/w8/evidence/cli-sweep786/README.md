# CLI sweep 786 — w8/135 retest at pin `ca30db3a3646` (live, 2026-10-19)

One-probe status check, nothing created.

`POST /registrycredentials` with the bex body still → 400 `invalid request body at /registry`: expected — the pin (`ca30db3a3646`) predates the w8/135 fix (`410b9ae7b`). The finding stays open until a pin passes that commit; the full live acceptance (create 201 + PATCH rename, sweep 676) will be re-run then.

No new bug filed.
