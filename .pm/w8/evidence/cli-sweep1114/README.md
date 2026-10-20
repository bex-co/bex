# CLI sweep 1114 — no numeric string coercion (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `port` as a string, `port` as a float, and `numInstances` as a string all → 400. The schema is type-strict: JSON numbers must be numbers, with no string/float coercion into integer fields.
