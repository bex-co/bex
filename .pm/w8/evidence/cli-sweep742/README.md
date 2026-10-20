# CLI sweep 742 — env-group service link dialect (live, 2026-10-19)

Fixtures `qa742-web` + `qa742-group` both deleted and verified absent (service 404, group list `[]`).

- Body-style link `POST /env-groups/{id}/services {"serviceId"}` → 404. Correct dialect is path-style: `POST /env-groups/{id}/services/{serviceId}` (no body) → 200, and the group read back `serviceLinks:[{id, name, type:"web"}]`.
- `DELETE /env-groups/{id}/services/{serviceId}` → 204, and 204 again on repeat (idempotent unlink).

No bug filed.
