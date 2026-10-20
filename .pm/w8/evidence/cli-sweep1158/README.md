# CLI sweep 1158 — registry credentials refuse incomplete creates (live, 2026-10-20)

Refusal-side only; nothing was created. No new bug filed.

- The credentials list for the workspace → 200 `[]`.
- A create with only a name → 400 `invalid request body at /username`: required fields are validated in order and named by JSON path, before any credential is stored.
- An unknown credential id → 404.
