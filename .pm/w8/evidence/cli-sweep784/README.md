# CLI sweep 784 — GitHub-integration read surfaces (live, 2026-10-19)

Read-only.

- `GET /owners/{id}/github`, `/owners/{id}/integrations`, `/github/installations` → 404 (no such REST surfaces; GitHub App state rides other routes).
- `GET /repos?ownerId=<canary>` → 200 `[]` (no repo-picker cache populated for the QA session).

No bug filed.
