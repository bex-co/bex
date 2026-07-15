# w2 · m37 — Create-payload completeness: `secretFiles` + `environmentId` at create

**Worker:** worker2 **Goal:** Render create bodies stop silently dropping fields: `servicePOST.secretFiles` seeds secret files at service create, and `environmentId` on service/Postgres/Key Value creates assigns the Environment (auto-joining its project) at birth instead of only post-create. **Status:** done

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | `secretFiles` at service create (three adapters)                | 45m | — **DONE** |
| t002 | `environmentId` at create: service + Postgres + Key Value       | 45m | — **DONE** |
| t003 | bex.yml acceptance for both fields                              | 30m | t001, t002 — **DONE** |
| t004 | Create-wizard secret-files step (or documented why-not)         | 40m | t001 — **DONE** |
| t005 | Render parity                                                   | 30m | t003, t004 — **DONE** |
| t006 | Simplify                                                        | 30m | t005 — **DONE** |
| t007 | Test coverage                                                   | 45m | t005 — **DONE** |
| t008 | Closeout                                                        | 15m | t007 — **DONE** |

## Definition of done

`POST /v1/services` (and the GraphQL/MCP creates) accepting `secretFiles: [{name, content}]` produces a service whose pods mount them at `/etc/secrets` from first boot. `environmentId` on the three direct creates lands the resource in that Environment with the project auto-join (ADR032 semantics), with 403/404 for a foreign/unknown Environment and identical behavior on all surfaces. Blueprint YAML follows Render's separate contract: membership is expressed by nesting resources under `projects[].environments[]`; direct `environmentId` and `secretFiles` are rejected by name because neither exists in Render's Blueprint resource schema. Omitting either direct-create field keeps the prior behavior byte-identical.

## Render evidence + closeout

- Render's create-service API and create flow expose initial secret files and Project/Environment selection ([Create a service](https://api-docs.render.com/reference/create-service), [Web services](https://render.com/docs/web-services), [Projects & Environments](https://render.com/docs/projects)). The service wizard therefore ships both controls.
- Render's [Blueprint specification](https://render.com/docs/blueprint-spec) and pinned [JSON schema](https://render.com/schema/render.yaml.json) have no direct service `secretFiles` or `environmentId`; canonical grouping is `projects[].environments[]`, with Key Value entries in `services` as `type: keyvalue`. bex mirrors that vocabulary and rejects create-body fields without echoing secret contents.
- Follow-ups filed: `w2/010` for environment-scoped Blueprint env-var groups and `w5/014` for Postgres/Key Value create-page Environment selectors.
- Live CAPD verification passed 2026-07-15: one service-create call produced an App at generation 1 whose first pod read its create-time file from `/etc/secrets`; the same Environment auto-joined its Project; omitted fields stayed ungrouped/secret-free; Postgres and Key Value were assigned at birth; an unknown Environment returned 404 before any App write. The verifier cleaned up all test resources.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 7, 2026-07-14 — systematic field-diff of Render's pinned OpenAPI: `servicePOST.secretFiles` (zero hits in bex's create path; the secret-files verbs themselves shipped w1/m16) and `environmentId` on `servicePOST`/`postgresPOSTInput`/`keyValuePOSTInput` (bex assigns environments only post-create via `set_environment_*`).
- **Goal linkage:** Render parity — w2's create-verification charter (m4 verified create field-by-field; w5/m20 closed the same asymmetry for `envVars`). A Render client sending either field today has it silently dropped, the exact failure mode the ledger exists to prevent.
- **Expected outcome:** create bodies match Render's contract; blueprint/agent flows can land a fully-configured service in one call; two fewer w7/m30 allowlist entries.
- **Why now:** both halves are thin threading over shipped verbs (secret-files w1/m16, environments w1/m32 + w6/m19/m20) — cheapest remaining parity; w2 owns the create contract. Render parity task included — all-surface change.
