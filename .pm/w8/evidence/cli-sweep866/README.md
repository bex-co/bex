# CLI sweep 866 — `autoDeploy` wire form (live, 2026-10-20)

Disposable free web service created, patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"autoDeploy":"yes"}` → 200, read back `autoDeploy: "yes"`, `autoDeployTrigger: "commit"` — the field is Render's `yes|no` string enum, not a boolean: `{"autoDeploy":true}` → 400.
- `autoDeployTrigger` is derived, not writable: PATCHing it directly → 400 (the trigger follows the autoDeploy value instead of being a second source of truth).
