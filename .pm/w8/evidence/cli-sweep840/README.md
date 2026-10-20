# CLI sweep 840 — platform maintenance runs are a documented non-goal (live, 2026-10-20)

Read-only probes. No new bug filed.

- `GET /maintenance` → 404, and per-service `/services/{id}/maintenance` → 404 (that path is not a Render operation at all). Render's platform maintenance-runs API (`/maintenance`, `/maintenance/{runId}`) is a recorded non-goal in [ADR018](../../../docs/ADR018-render-parity.md): "Maintenance runs | — | — | — | — | Render `/maintenance`. Managed-infra scheduling — non-goal." This is distinct from tenant-facing Maintenance Mode (w1/m37, ✅), which sweep 794 already exercised.
