# CLI sweep 871 — static-site buildCommand/branch/repo PATCH discipline (live, 2026-10-20)

Disposable static site created, patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"serviceDetails":{"buildCommand":"echo qa871-b"}}` → 200, read back at `serviceDetails.buildCommand` (the static spelling; not folded into `envSpecificDetails`).
- `PATCH {"branch":"qa871-branch"}` → 200 (branch is a plain editable field post-create, w5/m48).
- `PATCH {"repo":"not a url 871"}` → named 400 spelling out the accepted repo forms (https URL or GitHub SSH) — source changes are validated before any write, so a bad repo cannot strand the site.
