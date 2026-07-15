# w2 · m37 — Create-payload completeness: `secretFiles` + `environmentId` at create

**Worker:** worker2 **Goal:** Render create bodies stop silently dropping fields: `servicePOST.secretFiles` seeds secret files at service create, and `environmentId` on service/Postgres/Key Value creates assigns the Environment (auto-joining its project) at birth instead of only post-create. **Status:** todo

## Tasks (in order)

| id   | title                                                        | est | depends_on |
| ---- | -------------------------------------------------------------- | --- | ---------- |
| t001 | `secretFiles` at service create (three adapters)                | 45m | —          |
| t002 | `environmentId` at create: service + Postgres + Key Value       | 45m | —          |
| t003 | bex.yml acceptance for both fields                              | 30m | t001, t002 |
| t004 | Create-wizard secret-files step (or documented why-not)         | 40m | t001       |
| t005 | Render parity                                                   | 30m | t003, t004 |
| t006 | Simplify                                                        | 30m | t005       |
| t007 | Test coverage                                                   | 45m | t005       |
| t008 | Closeout                                                        | 15m | t007       |

## Definition of done

`POST /v1/services` (and the GraphQL/MCP creates) accepting `secretFiles: [{name, content}]` produces a service whose pods mount them at `/etc/secrets` from first boot; `environmentId` on the three creates lands the resource in that Environment with the project auto-join (ADR032 semantics), 403/404 for a foreign/unknown environment, identical on all surfaces; both fields work from `bex.yml`; omitting either keeps today's behavior byte-identical.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker` round 7, 2026-07-14 — systematic field-diff of Render's pinned OpenAPI: `servicePOST.secretFiles` (zero hits in bex's create path; the secret-files verbs themselves shipped w1/m16) and `environmentId` on `servicePOST`/`postgresPOSTInput`/`keyValuePOSTInput` (bex assigns environments only post-create via `set_environment_*`).
- **Goal linkage:** Render parity — w2's create-verification charter (m4 verified create field-by-field; w5/m20 closed the same asymmetry for `envVars`). A Render client sending either field today has it silently dropped, the exact failure mode the ledger exists to prevent.
- **Expected outcome:** create bodies match Render's contract; blueprint/agent flows can land a fully-configured service in one call; two fewer w7/m30 allowlist entries.
- **Why now:** both halves are thin threading over shipped verbs (secret-files w1/m16, environments w1/m32 + w6/m19/m20) — cheapest remaining parity; w2 owns the create contract. Render parity task included — all-surface change.
