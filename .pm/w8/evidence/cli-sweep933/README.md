# CLI sweep 933 — provenance/URL fields cannot be spoofed (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- PATCH `createdBy` / `createdAt` / `updatedAt` / `serviceDetails.url` → each 400 unknown-field by name; read-back shows the real creation timestamp and the platform URL (`https://qa933-web.onbex.co`) untouched. A client cannot backdate a service or point its published URL at an attacker host through the write model (extends sweep 914's server-owned-field discipline to provenance).
