# CLI sweep 667 — environment PATCH semantics (live, 2026-10-19)

Fixtures `qa667-proj`/`qa667-env` deleted and verified absent (`GET /projects?ownerId=…` → `[]`).

- `PATCH /environments/{id} {"name":…}` → 200, rename read back.
- `PATCH {"protectedStatus":"protected"}` → 200 (field accepted and validated against `protected|unprotected` per `lego/backend/internal/projects/service.go:437`).
- `DELETE` on the (empty) protected environment → 204. Consistent with the documented semantics: protection "blocks unguarded delete/suspend/direct-deploy-override on member services" (`lego/backend/internal/environments/mcp.go:63`) — it guards members, not the empty environment shell. The member-service guard itself is probed in sweep 668.
- `PATCH {"bogusField":1}` → 400 `request body contains unknown field "bogusField"` (strict, consistent with services).

No bug filed.
