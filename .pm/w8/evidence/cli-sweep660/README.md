# CLI sweep 660 — secret files and service events (live, 2026-10-19)

Disposable free web service `qa660-web` (`srv-dbbbf3avfpec73dm0jk0`). Deleted and verified absent (REST 404).

Secret files (`/services/{id}/secret-files`):

- Empty list → 200 `[]`. PUT one file → 200. PUT is replace-all (second PUT left only its own file listed), matching env-var semantics (sweep 647).
- Name `../evil.txt` → 400 `SECRET_FILE_INVALID` naming the config-key rule (path traversal refused).
- Non-base64 `content` (`!!!not-base64!!!`) was accepted and read back verbatim on the single-file GET: bex treats secret-file content as an opaque string with a consistent write/read round trip. Recorded, not filed — no pinned-oracle evidence that bex must enforce base64, and no decode failure was observed.

Events (`GET /services/{id}/events`): returned the creation deploy's `deploy_ended` event with deploy id/status/image/timestamps; `limit=0` → the same named `limit must be an integer between 1 and 100` refusal as sweep 651.

No bug filed.
