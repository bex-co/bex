# w21 · m584 — Adopt the bex Postgres operand image with each extension gated on its own, so PostGIS and the rest stop waiting on TimescaleDB's restore

**Worker:** worker21 **Goal:** production managed Postgres runs the bex operand image on every major 13–18; each extension whose own ADR092 gate passes is in the owner allowlist; `pg_repack`, `wal2json` and the `pgaudit` preload ride the same digest, so every managed cluster restarts once. **Status:** blocked — `pin must carry` `b8c7ac47b2` (t001–t004, t007, t008 done; t005's adoption commit shipped)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Amend ADR092: D3 property 5 is judged per extension; a failing extension stays out of the allowlist, keeps its preload, and joins later by reload — **DONE** | 30m | — |
| [t002](done/t002.md) | A per-extension gate: TimescaleDB and pg_ivm restore separately, `matrix.json` records each verdict, the allowlist takes only passed extensions, both architectures always run — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | One digest carries every image-level change: `pg_repack` on 16+ with its `repack` schema grants, `wal2json` on 13–18, `pgaudit` in `shared_preload_libraries` — **DONE** | 1h | t001 |
| [t004](done/t004.md) | Run the gate on 13–18 × arm64 and amd64, and record each digest and each extension's verdict — **DONE** | 45m | t002, t003 |
| [t005](t005.md) | Write the gated digests into `bexOperandImages` and restart every managed cluster once, off-peak, under ADR092 D4 | 1h | t004 |
| [t006](t006.md) | Production acceptance, and the w8 extension items told what they wait on now | 45m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 45m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

## Definition of done

1. `bexOperandImages` (`lego/operator/internal/controller/database_operand.go`) names a gated digest for every major 13–18 on `origin/main`, and every managed Postgres cluster in production runs it, each restarted once, with health read before and after and the rollback recorded.
2. `pg-operand-image.yml` published those digests: the gate ran both arm64 and amd64 for every major, `deploy/pg-operand/matrix.json` records a verdict for each extension, and the operator's allowlist equals the passed set (`TestPGExtensionAllowlistMatchesTheImageGate`).
3. On production, on a new Free database of one major below 16 and one of 16 or later, and on one existing database, the default user runs `CREATE EXTENSION postgis` and m92's original migration; `rolsuper` stays false; an extension outside the allowlist is refused; an extension that failed its gate (TimescaleDB at least) is refused with PostgreSQL's permission error.
4. On a 16+ database `pg_available_extensions` lists `pg_repack` 1.5.3; on every major a role holding `REPLICATION` creates a `wal2json` logical slot over the external URL and drops it; `SHOW shared_preload_libraries` lists `pgaudit`.
5. w8/m92, m104, m109, m110, m111, m112 and w8/276 name what they wait on now.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w21` (2026-11-23, Postgres `CREATE EXTENSION`), verified the same day:
  - Run `45019148603` (2026-11-22) failed on all six majors at one step only, the ordinary-owner restore at `deploy/pg-operand/check.sh:193`: `ERROR: permission denied to set parameter "timescaledb.restoring"`. Every allowlisted extension had been created and used before it, and the platform restore that follows passed.
  - The gate loop at `.github/workflows/pg-operand-image.yml:168` runs arm64 first under `bash -e`, so the amd64 leg, production's architecture, has never run.
  - 12 consecutive runs since 2026-11-04 failed, so nothing was ever published, and `bexOperandImages` is empty (`database_operand.go:36`).
  - This milestone carries the remaining half of w8/m92/t008 (qualification and publication) and w8/m92/t001's adoption. It does not replace the extension milestones' own acceptance.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); Render's [postgresql-extensions](https://render.com/docs/postgresql-extensions) ("To enable any supported extension, run the CREATE EXTENSION command"); [ADR092](../../../docs/ADR092-postgres-extension-supply.md); [ADR009](../../../docs/ADR009-postgresql-management.md).
- **Expected outcome:** a Render-shaped migration that runs `CREATE EXTENSION postgis` works on bex; w8/m92, m104 and m111 resume; later extensions join the allowlist by reload, with no second restart.
- **Why now:** six milestones and w8/276 have waited since 2026-10-31 on one gate step that only TimescaleDB (and possibly pg_ivm) fails.
- **Decision state.** The user's 2026-10-29 decision (ADR092: one bex image per major carrying all six extensions, owner-initiated `CREATE EXTENSION` through a closed allowlist, one rolling restart approved) stands unchanged. **Recommendation of the plan, recorded as a recommendation and not as a user decision:** judge ADR092 D3 property 5 per extension, so an extension that fails it stays out of the allowlist instead of holding back the image. It rests on AGENTS.md ("Never park parity work on an optional decision. Ship the parity-sufficient choice") and keeps ADR092 D3's rule against weakening a property: no allowlisted extension loses property 5. It moves toward Render, so no ADR018 sign-off is needed.
- **Render parity:** omitted. No REST, GraphQL, MCP or dashboard surface changes; the comparison with Render's extension table per major is in the definition of done.
- **Risks:**
  - pg_ivm's ordinary-owner restore has never run: it sits after TimescaleDB's failing step in the same chain. It may fail too and stay out.
  - Every major's Free gate container peaked at its 256 MiB limit (`cgroup_memory_peak_bytes=268435456`, page cache included). t002 reports anonymous memory so the Free plan's headroom is a number, not a guess.
  - pg_duckdb's own acceptance (file and network behavior) stays with w8/m109.
