# w4 · m242 — Env-var write responses and create inputs match Render: env-group writes answer the envGroup with values, and create-service accepts `generateValue`

**Worker:** worker4 **Goal:** every REST write that Render answers with the `envGroup` (create, set one variable, set one secret file) returns that object with its real values, every time. Render's create-service body accepts `{key, generateValue: true}` in `envVars`, as bex's own env-var endpoints already do. Today:

- most env-group creates answer every value as `""`;
- the per-variable and per-file PUTs answer a bare `{key}` / file view;
- `POST /v1/services` refuses `generateValue` as an unknown field.

**Status:** blocked — t001–t006 done 2026-10-25 (loopx w4); t007 waits for the release pin to include the fix, then QA replays the DoD in `bex-canary`

## Tasks (in order)

| id              | title                                                                                   | est | depends_on                 |
| --------------- | --------------------------------------------------------------------------------------- | --- | -------------------------- |
| [t001](done/t001.md) | Env-group create response: find why the reveal fails, and stop masking values as `""`   | 60m | —                          | — **DONE**
| [t002](done/t002.md) | Per-variable / per-file env-group PUT answers Render's `envGroup`                       | 40m | —                          | — **DONE**
| [t003](done/t003.md) | `POST /v1/services` (and the MCP create tools) accept `envVars[].generateValue`         | 40m | —                          | — **DONE**
| [t004](done/t004.md) | Render parity                                                                           | 20m | w4/m242/t001, w4/m242/t002, w4/m242/t003 | — **DONE**
| [t005](done/t005.md) | Simplify                                                                                | 15m | w4/m242/t004               | — **DONE**
| [t006](done/t006.md) | Test coverage                                                                           | 25m | w4/m242/t004               | — **DONE**
| [t007](t007.md) | Closeout (live DoD replay)                                                              | 15m | w4/m242/t005, w4/m242/t006 |

## Definition of done

Live in `bex-canary` after the release pin includes the fix, with an Admin session and in-page `fetch(..., {credentials:'include'})`. Every fixture is `qa-<date>-` prefixed:

- `POST /v1/env-groups` `{name, ownerId, envVars:[{key:"QA_LIT",value:"lit"},{key:"QA_GG",generateValue:true}]}`, repeated **10 times** → every `201` body carries `"QA_LIT"` with `"value":"lit"` and `QA_GG` with a 44-character base64 value. Today 4 of 7 creates answered `"value":""` for every key while an immediate `GET` showed the real values.
- `PUT /v1/env-groups/<id>/env-vars/QA_X` `{value:"x"}` → `200` with Render's `envGroup` (`id`, `name`, `envVars` including `{key:"QA_X",value:"x"}`, `secretFiles`, `serviceLinks`). Today: `{"key":"QA_X"}`. `PUT …/secret-files/<name>` answers the same `envGroup` shape.
- `POST /v1/services` (Free image web service) with `envVars:[{key:"QA_GEN",generateValue:true}]` → `201`, and `GET …/env-vars/QA_GEN` returns a 44-character value. Today: `400 unknown field "generateValue"`. `{key, value, generateValue:true}` together → `400`, as the per-var PUT already answers.
- Controls unchanged:
  - `PUT /v1/services/<id>/env-vars/QA_D` `{generateValue:true}` → `200` with a generated value.
  - Bulk `PUT …/env-vars` replaces the list and generates `QA_G`.
  - `PATCH /v1/env-groups/<id>` keeps returning values.
- A caller without `can_view_sensitive` still gets no values from these writes (the masked body omits `value` rather than sending `""`). Prove this by test; the QA workspace has no non-Admin member.
- Fixtures deleted (`GET` → `404`); session revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop run e9v4, 2026-10-24 UTC (`muse.env`, `bex-canary` `tea-daif693dqjvc73e7as3g`, pin `29d63e1f7bea`). Probe, root cause and blast radius in [finding.md](finding.md). Fixtures:
  - service `qa-20261024-e9v4-env` (`srv-dbe49mr17lic73al6qsg`);
  - groups `qa-20261024-e9v4-grp`, `-grp2`, `-grp3`, `-grp4`, `-r0`…`-r2`.
  - All were deleted (`204`, service `GET` → `404`); session revoked.
- **Goal linkage:**
  - Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first));
  - bex-api REST contract ([ADR006](../../../docs/ADR006-bex-api.md));
  - env-group values on write responses (w8/182, [ADR018](../../../docs/ADR018-render-parity.md) "Environment groups" row).
- **Expected outcome:** a Render-API client that creates a group, then reads `envVars[].value` from the response, gets the real values. A per-key PUT is decoded as the `envGroup` its generated client expects. A create-service manifest that asks Render to generate a secret works unchanged on bex.
- **Why now:** w8/182 claimed the create response carries values, but on production it usually doesn't, and nothing reports it. The fallback body sends `""` for every value, which a client cannot tell from a real empty value. A script that captures the response would silently store empty secrets. The Render parity closing task applies: REST, GraphQL and MCP create inputs change.
