# CLI sweep 669 — `autoDeploy` toggle dialects (live, 2026-10-19)

Disposable free image-based web service `qa669-web` (`srv-dbbbhn2vfpec73dm0k3g`). Deleted and verified absent (REST 404).

- `bex services update --auto-deploy` → 200; readback `"autoDeploy":"yes"`, `"autoDeployTrigger":"commit"` (stored even on an image service, where the trigger is inert — same field shape as Render).
- REST `PATCH {"autoDeploy":"yes"|"no"}` → 200 with exact readback; `{"autoDeploy":"bogus"}` → 400 `invalid request body at /autoDeploy`.

No bug filed.
