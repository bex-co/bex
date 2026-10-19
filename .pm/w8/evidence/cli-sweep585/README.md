# CLI sweep 585 — env groups: link, propagation to the process, group deletion cascade

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixtures: service `qa585-web` (`srv-dbb8llfo17is73furvgg`, free busybox serving `V585=$V585 SF=$(cat /etc/secrets/qa585.txt)`) and three env groups; all deleted at close (service 404, public URL 404, every group 404, zero `qa585` services). REST boundary via `/tmp/qa501/body.py` with the same QA authority (the CLI has no env-group verb).

- **A 403 that is correct tenant isolation, not a bug:** the first group was created without `ownerId`, so it landed in the caller's default workspace (`bex`, `tea-d98210cbbpdc73dcrkvg`), not `bex-canary`. Linking it to the canary service returned bare `403 FORBIDDEN` — the tenant check in `LinkService` ([service.go](/lego/backend/internal/envgroups/service.go:1031)) refusing a cross-workspace link, with no existence leak. Recreating the group with `ownerId: tea-daif693dqjvc73e7as3g` linked immediately (`serviceLinks` populated). Lesson recorded: REST probes must send `ownerId` explicitly; the `X-Bex-Workspace` header alone does not move env-group creation.
- **Group contents reach the process (closes sweep 578's Unrun):** after linking, the service's public page read `V585=one SF=gruppo585` — both the group env var and the group secret file (`/etc/secrets/qa585.txt`) materialized. Updating the group var to `two` rolled the service and the page read `V585=two` within ~30s.
- **CRUD dialects:** duplicate group name → `409 ENV_GROUP_NAME_EXISTS`; missing name → `400 invalid request body at /name`; group names are free-form labels (`QA585_Bad!` accepted on create and rename — env groups are not resource-name addressed); bulk var PUT replaces, per-key GET reveals the value, missing key → `404 NOT_FOUND`; secret-file PUT echoes `{name}` only, GET reveals content; synthetic group/service ids → `404`.
- **Deletion cascades honestly:** deleting a still-linked group returned `204`; the service rolled once more and its page read `V585= SF=` — values withdrawn, service unharmed. A later unlink attempt on the deleted group → `404`.
- **Filter note:** `GET /env-groups?name=qa585` (prefix of the then-current name) returned `[]`; the filter is not a prefix search. Exact-name behavior was not isolated — see Unrun.

## Unrun

- `?name=` exact-match vs substring semantics, and list pagination cursors on env groups.
- A service's own env var shadowing a group var of the same key (precedence).
- Group secret file colliding with a service-owned secret file of the same name.
- Linking a second service to the same group (multi-service fan-out).
