# w4 · m137 — A restarting Key Value or Postgres reports "creating", as if it were brand new

**Worker:** worker4 **Goal:** a datastore that has been Available, then restarts (a config change, a manual restart, or a rollout), reports Render's restart status instead of `creating` on REST, GraphQL, MCP, and the dashboard, and never reports `available` while its restart is already underway **Status:** todo

## Tasks (in order)

| id   | title                                                                                         | est | depends_on       |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Key Value: `kvStatus` distinguishes a restart from creation, and a stale Ready from a live one | 30m | —                |
| t002 | Postgres: `dbStatus` distinguishes a restart from creation                                     | 30m | —                |
| t003 | Dashboard: map the restart status on both datastores, and name the restart in KV config copy   | 30m | t001, t002       |
| t004 | Render parity across REST / GraphQL / MCP / UI                                                | 20m | t003             |
| t005 | Simplify                                                                                      | 15m | t004             |
| t006 | Test coverage                                                                                 | 30m | t004             |
| t007 | Closeout                                                                                      | 10m | t006             |

## Definition of done

Each bullet is a probe that was run at filing and can be repeated on production with a throwaway `qa-<date>-` resource.

- **Key Value config change.** Create a Free public Key Value and wait for Available. Poll `redis-cli --tls --sni <host> -u <external URL> CONFIG GET maxmemory-policy` and GraphQL `keyValue(id:){status}` every 4s, then change Maxmemory Policy. While `redis-cli` reports `SSL_connect failed`, status is `config_restart` (UI: "Restarting"), never `creating`, and never `available`. At filing (pass 159, `red-darktcjbdpcs73f5eha0`, times UTC): status was `available` from 04:57:43 to :57 and `creating` from 04:58:01 to :17, while the store was unreachable from 04:57:43 to 04:58:22.
- **Postgres manual restart.** Create a Free public Postgres, poll `psql "<external URL>" -c 'select 1'` (with `PGSSLROOTCERT` set to the downloaded CA) and GraphQL `database(id:){status}` every 3–4s, then Restart Database. While psql is refused, status is the same restart value, never `creating`. At filing (pass 160, `dpg-darl85bthimc73a1m01g`): `creating` in both UI and GraphQL from 05:17:32 to 05:18:19, and psql refused from 05:17:35 to 05:18:16.
- **Creation still says creating.** A brand-new store of either kind reports `creating` until its first Ready (the control case, observed correct in both passes).
- **The dashboard never shows "Unknown" for the new value.** List rows, project pages, and the detail header all label it.

## Source + Goal linkage

- **Source:** promoted from `w4/141` ([source-141.md](source-141.md)), the Key Value half from live `/qa-find-bugs` pass 159, 2026-09-25. The Postgres half was reproduced in pass 160 the same day (w4-targeted, `muse.env` credentials, journey 11) on `qa-20260925-pg`, which was created, exercised, and deleted within the pass. That pass also confirmed the restart dialog's copy is honest ("Connections drop briefly while it comes back"). Only the status word is wrong.
- **Root cause (both halves, same shape):**
  - `lego/backend/internal/keyvalue/service.go:225-234` `kvStatus` and `lego/backend/internal/postgres/service.go:381-392` `dbStatus` default every non-Ready phase to `"creating"`.
  - The operators reuse the `Provisioning` phase for any not-ready state after first readiness: `lego/operator/internal/controller/keyvalue_controller.go:846` and `database_controller.go:1305` (`Reason: "Provisioning"`, `"waiting for CloudNativePG"`).
  - Each mapper has exactly one caller: `kvView` (`keyvalue/service.go:241`) and `pgView` (`postgres/service.go:416`). Those views feed REST, GraphQL, and MCP.
- **Target behavior:**
  - For a store that has been Ready at least once, a non-Ready, non-Failed phase maps to **`config_restart`**. This is Render's `databaseStatus` value for "a config change restarted the instance", and bex already emits the matching `key_value_config_restart` event (`docs/render-artifacts/datastore-webhook-events.md:77`).
  - For Key Value, a `Ready` phase whose Ready condition's `ObservedGeneration` trails `Generation` also maps to `config_restart`. That closes the stale-`available` window.
  - `creating` is reserved for never-Ready stores. Precedence stays `deleting` > `suspended` > the rest.
  - "Has been Ready" must come from status the operator already persists (for example, KV `Status.CredentialRevision`, which is set only on Ready). Postgres needs its own equivalent, which t002 must identify from `DatabaseStatus` rather than invent a field without checking.
- **Consumer check:** both dashboard `STATUS_MAP`s (`dashboard/src/features/keyvalue/lib/status.ts`, `dashboard/src/features/databases/lib/status.ts:94`) fall back to `unknown` for unmapped values. The backend and dashboard changes must land together. `databases/lib/status.ts:121` treats `creating`/`upgrading` as transitional (it drives polling), so `config_restart` must join that predicate or the badge stops refreshing mid-restart.
- **Goal linkage:** ADR009 / ADR021 (managed data), ADR018 datastore rows, ADR006 (one view serves three surfaces).
- **Expected outcome:** an operator watching a store restart sees "Restarting", not a store being created from scratch, and a CLI or agent polling `status` can tell a restart from a first provision.
- **Why now:** the same default-to-`creating` shape now has live evidence on both datastores, and both one-caller mappers can be fixed in one small change before more status consumers are added.
- **Render parity task included:** `status` values change on REST/GraphQL/MCP and the dashboard labels change.

## Unverified

- **What Render reports for a user-initiated Postgres restart** (as opposed to a config change) was not observed. `config_restart` is the closest value in the pinned enum (`lego/backend/internal/api/openapi/render-public-api-1.json` `databaseStatus`). t004 must confirm it or pick the documented alternative (`unavailable`).
- `dbStatus` also returns `upgrading`, which is **not** in Render's enum. It was not exercised this pass. t004 records it as parity drift rather than folding it in.
- Key Value instance-type and persistence-mode changes go through the same mapper but were not exercised (instance type is a paid change).
