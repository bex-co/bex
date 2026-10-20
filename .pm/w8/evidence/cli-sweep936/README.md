# CLI sweep 936 — scale body type discipline (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `numInstances` as string `"2"`, float `1.5`, or missing → each 400 `invalid request body at /numInstances` (no string coercion, no fractional truncation, no default) — extends sweep 796's bounds checks to type discipline; an extra sibling field → unknown-field refusal by name.
