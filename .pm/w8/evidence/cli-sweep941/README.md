# CLI sweep 941 — timestamp monotonicity at birth and after PATCH (live, 2026-10-20)

Disposable free web services created, patched, deleted (204s). No new bug filed.

- At birth `createdAt` and `updatedAt` are the same instant rendered at different precisions (`…26Z` vs `…26.376813082Z` — created is second-truncated, updated nanosecond): cosmetically asymmetric but parse-identical, recorded rather than filed.
- After a rename PATCH, `updatedAt` strictly advances while `createdAt` stays bit-identical — update monotonicity and creation stability both hold.
