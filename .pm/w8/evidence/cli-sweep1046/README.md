# CLI sweep 1046 — Key Value suspend/resume (live, 2026-10-20)

One disposable free Key Value instance created and deleted (204; GET 404). No new bug filed.

- KV suspend → 202 with `suspended: suspended`; resume → 202; a rename PATCH in between → 200. Datastore suspension follows the same control-plane-open pattern as services (sweep 1032).
