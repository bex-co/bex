# CLI sweep 562 — deploy-history paging past the default page (26-row fixture)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T16:23–16:33Z. Owned fixture `qa562-paging` (`srv-dbb484vo17is73furqog`) created, exercised, deleted via CLI, and proven absent (REST 404; workspace back to 8).

No baseline service had more than 2 deploys, so the fixture manufactured a paging-length history: after reaching `live`, 24 back-to-back `bex deploys create` calls each opened a row (26 total with the two setup deploys). The burst settled truthfully: newest `live`, 24 `canceled` (superseded burst rows plus the discarded first image's deploy), 1 `deactivated` (the previously-live release). The public URL then served `200 Hello, world!`.

Paging checks (full-body same-credential REST, `/tmp/qa501/paging562.py`, because the pinned `deploys list` exposes no `--limit`/`--cursor` flags):

- `?limit=100` returned all 26 rows as `{deploy, cursor}` pairs, strict `createdAt`-descending, ids unique.
- Cursor walk at the default size: page 1 = 20 rows, page 2 (via row 20's cursor) = 6 rows, concatenation byte-identical in order to the `limit=100` read; one page past the end = `[]`.
- A `limit=7` walk (4 pages) reproduced the same 26-id order — no duplicates, no skips at either page size.
- `bex deploys list` returned exactly the newest 20 (page 1), newest `live`.
- Limit boundary (on baseline hello-go): `limit=0/101/999/-1` → named `400 limit must be an integer between 1 and 100`; `limit=abc` → `400 invalid query parameter "limit"`; `cursor=garbage` → `200 []` (lenient empty page); empty cursor behaves as no cursor.

Setup detour worth recording (harness error, exemplary product diagnosis): the first fixture image (`ghcr.io/traefik/whoami`) listens on port 80 while the service port defaulted to 3000, so the rollout stalled — and the deploy's `stallReason`/event said exactly that ("its startup health check has not succeeded … a TCP connect to port 3000"). The pinned CLI has no `--port` update flag (Render's surface likewise), and REST `PATCH port: 80` refused by name ("port must be 1024-65535 — a tenant container has no NET_BIND_SERVICE…"). Swapping to a `$PORT`-honoring image (`gcr.io/google-samples/hello-app:1.0`) went `live` in ~20 s.

## Unrun

- Paging while rows are being appended mid-walk (cursor stability under concurrent creates).
- Deploy-list paging beyond 100 rows (the REST cap — a >100-row history needs 100+ creates; the two-page and 7-page walks already exercise the keyset contract).
