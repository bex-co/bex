# CLI sweep 662 — `whoami` and output-format validation (live, 2026-10-19)

Read-only.

- `bex whoami` (text) and `-o yaml` both return the QA account email and id consistently (`name` empty in both).
- `-o bogus` on `whoami` and on `services` → `invalid output format: bogus` before any request.

Format validation is uniform across commands. No bug filed.
