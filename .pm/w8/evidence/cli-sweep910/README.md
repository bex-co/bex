# CLI sweep 910 — request-body size ceiling (live, 2026-10-20)

Refused creates only; name list confirms nothing was created. No new bug filed.

- A ~3 MB JSON body → 413 `request body exceeds 2097152 bytes` with `id: payload_too_large` — a named 2 MiB ceiling enforced before validation (both a bogus-field body and a schema-shaped body hit it identically, so size is checked ahead of field discipline).
