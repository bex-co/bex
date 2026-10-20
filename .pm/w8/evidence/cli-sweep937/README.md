# CLI sweep 937 — deploy `imageUrl` tag/digest forms (live, 2026-10-20)

Disposable free image-backed web service created, deployed three ways, deleted (204, then GET 404). No new bug filed.

- `clearCache: "clear"` → 201 (the second enum value beside sweep 799's `do_not_clear`).
- Same-repository `imageUrl` with a new tag (`:2.0`) and with a full `sha256:` digest → 201 each (the per-deploy origin flexibility sweep 833's host-lock allows).
- A malformed digest (`@sha256:zzzz`) → named 400 `image digest is invalid: invalid checksum digest length` — digest syntax is validated, not passed through to fail at pull time.
