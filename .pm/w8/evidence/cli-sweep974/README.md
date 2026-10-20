# CLI sweep 974 — members, invites, notification-settings paths (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- `GET /owners/{id}/members` → the workspace roster (the QA user as `ADMIN`, `status: active`); unknown owner → 403, consistent with the owner boundary.
- Invites live at `/workspaces/{id}/invites` (200, empty); the guessed `/owners/{id}/invites` 404 is the correct absence of that route shape.
- `GET /notification-settings/owners/{id}` → 404: workspace-level notification settings have no such route; the registered family is the per-service `overrides` routes (the deliberate ADR018 posture already recorded for the POST side). Only the per-service override shape from [w8/138](../../138.md) exists here.
