# w21 · m586 — `CREATE EXTENSION vector;` works as Render documents it: a new database gets pgvector on demand, not preinstalled

**Worker:** worker21 **Goal:** on a new database the default user runs Render's literal `CREATE EXTENSION vector;` and it succeeds, and can `DROP EXTENSION vector`; existing databases keep the vector they have. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Amend ADR092 D2 and ADR009 § pgvector: a new database gets `vector` on demand; an existing database keeps its installed extension | 30m | w21/m584/t001 |
| [t002](t002.md) | Admit `vector` on every major; the gate runs Render's literal `CREATE EXTENSION vector;` and the owner's `DROP EXTENSION vector` | 45m | t001 |
| [t003](t003.md) | Operator: a new database gets no vector Database child and no `InitializingExtensions` wait; an existing database's child is released without dropping `vector` | 1h | t002 |
| [t004](t004.md) | Production: Render's command on a new database exits 0, and an existing database's vectors and HNSW index are unchanged | 30m | t003, w21/m584/t005 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

## Definition of done

1. On production, on a new Free database of one major below 16 and one of 16+ (disposable), `psql -v ON_ERROR_STOP=1 -c 'CREATE EXTENSION vector;'` as the default user exits 0, a `vector(3)` column with an HNSW index answers a nearest-neighbour query, and `DROP EXTENSION vector` by the same user succeeds.
2. A new database reports its first `available` without waiting for an extension (no `InitializingExtensions`) on REST, GraphQL, MCP and the dashboard.
3. An existing database that had `vector` preinstalled still has it after the operator change, with its rows and indexes unchanged, and no reconcile drops it.
4. ADR092, ADR009 and the ADR018 Postgres row describe on-demand `vector` and the residual on existing databases.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w21` verification (2026-11-23). Render's extension table says "Enable this extension with `CREATE EXTENSION vector;`" ([postgresql-extensions](https://render.com/docs/postgresql-extensions)). bex installs `vector` in every application database through a CloudNativePG Database child (`lego/operator/internal/controller/database_extensions.go:85`, ADR009 § pgvector, w8/m84). PostgreSQL's `CREATE EXTENSION` without `IF NOT EXISTS` errors when the extension exists, so Render's documented command fails on bex with `extension "vector" already exists`, and a migration run with `ON_ERROR_STOP` stops there. The extension belongs to the superuser that installed it, so the owner's `DROP EXTENSION vector` is refused ("must be owner"). ADR018's Postgres CRUD row records "pre-installed rather than on-demand" without a sign-off. No board item names either failure.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); [ADR092](../../../docs/ADR092-postgres-extension-supply.md) D2; [ADR009 § pgvector](../../../docs/ADR009-postgresql-management.md#pgvector-in-every-application-database-w8m84).
- **Expected outcome:** a Render user's pgvector setup runs unchanged on bex, and new databases become available sooner (no extension wait, w4/m223).
- **Why now:** preinstalling was the only way to give a non-superuser `vector` before the allowlist; w21/m584 makes the allowlist live, so the reason for the divergence ends there.
- **Decision state.** **Recommendation of the plan, not a user decision:** amend ADR092 D2's "`vector` keeps its existing preinstall" so new databases follow Render. It rests on AGENTS.md ("When an ADR … conflicts with parity, amend that ADR"). The user accepted ADR092 on 2026-10-29 with that line, so the amendment is named here for the user to see; it needs no ADR018 sign-off because it removes a divergence.
- **Residual, recorded rather than fixed:** an existing database keeps a superuser-owned `vector` (PostgreSQL has no way to hand an extension to another owner), so Render's plain `CREATE EXTENSION vector;` still errors there. t001 records it in ADR018.
- **Risk:** a bex user whose flow relied on `vector` being present in a new database without running `CREATE EXTENSION` sees `type "vector" does not exist`. That is Render's behavior.
- **Render parity:** included. The first-`available` timing of a new database changes on REST, GraphQL, MCP and the dashboard.
