# CLI sweep 932 — every verb on a nonexistent service converges to the same 404 (live, 2026-10-20)

Session re-verified (`whoami` → QA identity); pin unchanged (`3ede47d3c5a9`). Disposable fixture created and deleted (204, then GET 404). No new bug filed.

- Subresource read (`env-vars`), PATCH, and `suspend` against a synthetic service id, a malformed id (`not-an-id`), and a slash-encoded id all → the identical coded 404 `NOT_FOUND` envelope — no verb leaks a different existence signal (no 400-vs-404 split an enumerator could use to distinguish "bad shape" from "gone").
