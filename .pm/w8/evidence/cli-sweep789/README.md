# CLI sweep 789 — env-var `generateValue` + secret-files cycle (live, 2026-10-20)

Disposable free web service created via REST, exercised, deleted (204, then GET 404). No new bug filed.

- `PUT /services/{id}/env-vars` with one `{"key":…,"generateValue":true}` and one plain `value` entry → 200; the generated entry came back with a 44-char generated value (not recorded here), the plain entry verbatim. Single-var GET returns the same generated value (length-checked only).
- Secret files: `PUT` list → 200 echo; single GET → 200 with content; DELETE → 204; GET after delete → 404 `secret file not found` (named, coded — not a bare 500).
