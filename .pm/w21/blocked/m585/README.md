# w21 · m585 — Every contrib extension on Render's list can be created by the database owner, and `pg_repack` works for a table's owner

**Worker:** worker21 **Goal:** on each major 13–18 the default user runs `CREATE EXTENSION` for every contrib module Render lists for that major (`dblink`, `earthdistance`, `moddatetime`, `bloom`, `pgaudit` and the rest), admitted by a reload of the allowlist with no restart, and a table's owner repacks it with `pg_repack -k` on 16+. **Status:** blocked — `pin must carry` `979d68fdc1` (t001, t002, t005 and t006 done; t004's gate half done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record a disposition per extension in ADR092: the majors Render lists it on, PostgreSQL's default grants kept, and the `dblink` and `file_fdw` review — **DONE** | 1h | w21/m584/t001 |
| [t002](done/t002.md) | The allowlist and the gate: the admitted lists per major, `check.sh`'s refusals of `file_fdw`, `dblink` and `adminpack` turned into creations, and a non-listed untrusted extension still refused — **DONE** | 1h | t001 |
| [t003](t003.md) | Reload the allowlist on production and run w8/276's acceptance | 45m | t002, w21/m584/t005 |
| [t004](t004.md) | `pg_repack -k` as a table's owner on 16+ over the external URL, in the gate and on production | 30m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 40m | t004 |
| [t007](t007.md) | Closeout | 10m | t005, t006 |

## Definition of done

1. On production, on a new Free database of each major 13–18 (disposable), the default user's `CREATE EXTENSION` succeeds for every extension Render lists for that major and that is not trusted: `amcheck`, `autoinc`, `bloom`, `dblink`, `dict_xsyn`, `earthdistance`, `file_fdw`, `insert_username`, `intagg`, `moddatetime`, `pageinspect`, `pg_buffercache`, `pg_freespacemap`, `pg_prewarm`, `pg_surgery` (14+), `pg_visibility`, `pgaudit`, `pgrowlocks`, `pgstattuple`, `refint`, `sslinfo`, `xml2`, plus `adminpack` (13–16) and `old_snapshot` (14–16).
2. `CREATE EXTENSION cube; CREATE EXTENSION earthdistance; SELECT earth_distance(ll_to_earth(0,0), ll_to_earth(0,1));` returns a distance, and a `moddatetime` trigger updates a timestamp column.
3. The user is still not a superuser, an untrusted extension not on Render's list is still refused, and a logical export and restore of a database using these extensions works.
4. On 16+, `pg_repack --no-superuser-check --table=<t>` run by the table's owner over the external URL repacks it and leaves its rows unchanged.
5. No managed cluster restarted for this milestone (the admission was a reload).

## Source + Goal linkage

- **Source:** w8/276 (`/qa-find-bugs-cli` sweep `05f7`, 2026-11-11, production, PostgreSQL 18): 22 extensions on Render's list answer `42501 permission denied … Must be superuser`, and `pg_repack` is not installed. Reshaped by `/pm-brainstorm one aspect of render parity for w21` (2026-11-23) and verified that day: the gate itself asserts that `file_fdw`, `dblink` and `adminpack` stay refused (`deploy/pg-operand/check.sh:60-61`, `:176`); `adminpack` and `old_snapshot` left PostgreSQL in 17. This milestone replaces w8/276.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first); Render's [postgresql-extensions](https://render.com/docs/postgresql-extensions) and [postgresql-pg-repack](https://render.com/docs/postgresql-pg-repack) (read 2026-11-23); [ADR092](../../../../docs/ADR092-postgres-extension-supply.md).
- **Expected outcome:** a migration that runs `CREATE EXTENSION dblink` or `earthdistance` succeeds on bex as on Render; a bloated table is repacked by its owner without a long lock.
- **Why now:** a third of Render's list fails today; w21/m584 makes the allowlist mechanism live, after which this is a reload.
- **Decision state.** **Recommendation of the plan, not a user decision:** admit each module with PostgreSQL's default grants unchanged (so inspection modules such as `pageinspect` and `pg_surgery` install but their functions stay superuser-only, as PostgreSQL ships them), which needs no image change. Render does not document what its users can call after creating them; granting more is a follow-up, not a gate. It rests on ADR008 (parity first) and ADR092 D3 property 2 (no superuser leak).
- **Sequence constraint:** an extension that turns out to need an after-create grant script needs an image change, so t001 runs before w21/m584 t004 publishes, or that extension waits for the next image.
- **Render parity:** omitted. No REST, GraphQL, MCP or dashboard surface changes; the comparison with Render's table per major is the definition of done.
- **Review facts verified 2026-11-23:** a database pod's egress is the public internet and its own workspace only ([ADR043](../../../../docs/ADR043-tenant-namespace-isolation.md) D8.3); a non-superuser's `dblink` connection must authenticate with a password, so the pod's local peer trust is out of reach; `file_fdw` reads a file only for a holder of `pg_read_server_files`, and the default user has no `CREATEROLE` ([ADR009](../../../../docs/ADR009-postgresql-management.md), w8/257). Granting `CREATEROLE` later reopens the `file_fdw` review.

## Parked 2026-11-23 (`/loopx w21`)

- **Shipped:** `979d68fdc1` admits Render's 24 untrusted extensions per major in `admittedExtensions()` and `matrix.json` `admitted`, with PostgreSQL's own grants and no image change; ADR092 D1 § Render's untrusted contrib modules records each disposition, the `dblink` and `file_fdw` review and the `CREATEROLE` reopen condition; the gate creates, uses and judges each one, and checks the owner's `pg_repack` (rows, space, another role's table refused). Local gate on the six pinned digests: every major `PASS`.
- **Decision (the plan's recommendation, not a user decision):** `pgaudit` is admitted although CloudNativePG's instance manager drops it from every database at its next start on a primary while no `pgaudit.*` parameter is set (v1.30.1 `reconcileDatabases`); a `pgaudit.*` parameter would preinstall it everywhere instead, which ADR092 D2 rules out. Keeping it across restarts is [w21/122](../../done/122.md), promoted to [w21/m606](../../m606/README.md).
- **Gate:** production's pin must carry `979d68fdc1` (which contains m584's `b8c7ac47b2`). On 2026-11-23 at 21:30Z the pin was `9fdf7e0a1171`, which carries neither.
- **Next (t003, then t004's production half, then t007):**
  1. Confirm every managed cluster runs the bex operand image (w21/m584 t005 records the adoption) and `SHOW extwlist.extensions` lists the 24 names on its major.
  2. Restarts: if the first pin carrying `979d68fdc1` also brought `b8c7ac47b2`, each cluster restarted once for m584's adoption and the list arrived with it, so definition-of-done item 5 is "no restart beyond m584's one" and the reload path is the gate's (`check.sh` widens the list by a reload). If an earlier pin had already adopted the image, read each cluster's postmaster start time before and after this pin: unchanged.
  3. On a new Free database per major 13–18 in `bex-canary` (disposable): every `CREATE EXTENSION` of definition-of-done item 1, `cube` + `earthdistance` + `earth_distance`, a `moddatetime` trigger, `rolsuper` false, `CREATE EXTENSION postgres_fdw` refused, and an export and its restore; then on a 16+ one, `pg_repack --no-superuser-check` 1.5.3 over the external URL (rows compared, another role's table refused). Record in ADR092 and w8/276, delete the databases (404).
