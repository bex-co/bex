# CLI sweep 915 — Key Value server-owned fields cannot be spoofed (live, 2026-10-20)

Disposable free Key Value instance created, probed, deleted (204, then GET 404). No new bug filed.

- PATCH `ownerId` / `id` / `status` → 400 unknown-field refusals (the KV message nests the strict-decode prefix twice — `bad request: bad request body: request body contains…` — a cosmetic stutter in the error text only; the refusal, code, and no-write outcome are all correct, and the owner read-back is intact). Recorded as a text nit, not filed as a behavior bug.
