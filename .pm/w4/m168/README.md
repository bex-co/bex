# w4 · m168 — Loop6: bound env/secret-file key length so a long name can't wedge a service's environment

**Worker:** worker4 **Goal:** an over-long env key or secret-file name is a clear 400 on every surface, and no failed environment write can leave stored state that differs from what is mounted or that cannot be deleted **Status:** todo

## Tasks (in order)

| id   | title                                                          | est | depends_on                 |
| ---- | -------------------------------------------------------------- | --- | -------------------------- |
| t001 | Reject env keys and secret-file names over 253 characters      | 1h  | —                          |
| t002 | Single-key writes restore the store when projection fails      | 1h  | —                          |
| t003 | Empty-prior environment compensation actually restores         | 45m | —                          |
| t004 | Break the single-delete deadlock on unprojectable names        | 45m | w4/m168/t002               |
| t005 | Blast radius of the validator change across all callers        | 30m | w4/m168/t001               |
| t006 | Render parity across environment write surfaces                | 30m | w4/m168/t003, w4/m168/t004, w4/m168/t005 |
| t007 | Simplify changed code                                          | 20m | w4/m168/t006               |
| t008 | Test coverage for shipped behavior                             | 45m | w4/m168/t006               |
| t009 | Closeout                                                       | 15m | w4/m168/t008               |

## Definition of done

Each check is a probe you can repeat from the dashboard origin (page `fetch` with `credentials:'include'`) on owned Free `qa-` fixtures:

- **Group, 254 refused / 253 kept:** on a fresh env group, dashboard Edit → Add secret file with a 254-character name → Save is refused before submit with a message that states the length limit. REST `PUT /v1/env-groups/<id>/secret-files/<254 chars>` returns **400** naming the length rule (today: 409 `ENV_GROUP_UPDATE_RESTORED`, "the environment group update failed and its previous state was restored"). A 253-character name still saves and deploys (today's control).
- **Service with no prior files:** `PUT /v1/services/<id>/secret-files/<254 chars>` and GraphQL `patchServiceEnvironment` carrying one 254-character file both return a 400 (today: REST 500 `internal_error`, GraphQL "internal error"). `GET /v1/services/<id>/secret-files` afterwards returns `[]` (today the 254-character name is listed).
- **Service env var:** `PUT /v1/services/<id>/env-vars/<254-character key>` returns 400 and the key is not listed (today: 500 and the key persists; a following `PUT .../env-vars/NORMAL` also returned 500 yet wrote NORMAL to the store).
- **Recovery:** in a backend test harness (not prod), seed a service store with two unprojectable names. Deleting each through `DELETE /v1/services/<id>/secret-files/<name>` returns 204, then `PUT .../secret-files/ok.txt` returns 200 and the listing matches the projected Secret.
- **Source == projection:** after any failed environment write, the REST/GraphQL listing equals the projected env/files Secrets. No file is listed but unmounted, like today's `ok.txt`.

## Source + Goal linkage

- **Source:** infinite `/qa-find-bugs` loop6 2026-10-04 UTC (muse.env, `bex-canary` workspace), journeys 2, 4 and 16, researched at HEAD `5468ec870`. Fixtures (all deleted, zero `l6` residue):
  - `srv-db1e42oa8tcs73b1a0bg`, the wedged service. Its store held `ok.txt`, one 253-character name and two 254-character names. REST deletes of each returned 500 and removed nothing. The dashboard's one-save delete of both 254-character names recovered it.
  - `srv-db1ebs8a8tcs73b1a0q0`, the isolation fixture. With no prior files, a failed patch persisted the bad name. With one prior file, the same patch restored correctly. A 254-character env key also returned 500, persisted, and blocked later env writes until deleted.
  - Group `evg-db1e360a8tcs73b1a0ag`: a 253-character name saved; 254 returned 409 RESTORED.

  Evidence (local, gitignored): `.playwright-mcp/qa-envgroup-longname-1.png`, `.playwright-mcp/qa-svc-secretfile-wedged-1.png`. Governing ADRs: ADR013 (secrets) and ADR006 (env groups / secret-files API).
- **Goal linkage:** ADR008 reliable Render-alternative hosting, with truthful environment state: what the API lists is what the process gets.
- **Expected outcome:** an over-long name is a clear 400 everywhere (UI/REST/GraphQL/MCP). No save reports failure yet persists, and no stored state becomes impossible to delete.
- **Why now:** any tenant can reach this from the dashboard editor by typing a long file name. Two such attempts permanently break single-file REST/MCP management of that service, and its listings then misreport what is mounted. Render parity task INCLUDED (t006): REST/GraphQL/MCP/UI environment write surfaces change and must agree.
- **Severity:** major.
- **Unverified (carried from the hunt):** MCP tools (same service functions, not probed). Create-time `secretFiles`/`envVars` on `createService` and blueprint env keys over 253 (not probed). The exact cause of the empty-prior restore failure (t003 must confirm it from logs).
