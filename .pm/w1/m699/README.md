# w1 · m699 — A variable holding a bex datastore's external URL in the service's own region offers "Use Internal URL", as Render's editor does

**Worker:** worker1 **Goal:** while a service's variables are edited, a value holding the external URL of a Postgres or Key Value in the same workspace and region shows Render's warning, and one click rewrites it to the internal URL. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's External URL warning and bex's URL shapes in `docs/render-artifacts/service-environment-page.md` | 30m | — |
| [t002](t002.md) | GraphQL gives a Postgres's internal hosts without a secret view | 45m | t001 |
| [t003](t003.md) | Find a datastore's external URL in a value, and build its internal URL | 1h15m | t002 |
| [t004](t004.md) | The warning, "Review fix" and Update in the service's editor | 1h | t003 |
| [t005](t005.md) | Locales, and a walk with a Postgres and a Key Value on dev-1 and production | 1h15m | t004 |
| [t006](t006.md) | Render parity | 20m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006 |
| [t009](t009.md) | Closeout | 10m | t007, t008 |

Total: **9 tasks, about 6h**.

## Definition of done

1. While a service's Environment Variables are being edited, a value holding the external URL of a Postgres (its primary, pool or read replica host) or a Key Value in the workspace, whose region is the service's, shows Render's warning and its **Use Internal URL** button.
2. **Use Internal URL** opens "Review fix" with the old and new values. **Update** puts the new value in the draft, and the editor's usual Save saves it. Cancel changes nothing.
3. The new value keeps the pasted credentials, database and query, except `sslmode=verify-full`, and it connects from the service. Postgres uses its internal host. Key Value uses `redis://<id>:6379`, with the credentials kept when its internal auth is on.
4. There is no warning for:
   - a URL whose datastore is not in the workspace, or is in another region;
   - secret files, environment groups or static sites;
   - a page that is not being edited.
5. All of the above walked on production at 1440 and 390 px, in all eight locales, with a disposable Postgres and Key Value.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 3, verified before filing.
  - It is split out of [w1/154](../blocked/154.md), which recorded this warning beside Render's Datastore URL picker. w1/154 found that `.pm/DO_NOT_DO.md`'s "Link database to service" line does not cover it: the line forbids a picker that inserts a connection string, and this warning picks nothing, it rewrites a value the user typed. The picker stays parked on the user's decision, and this milestone does not need it.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`, 459 chunks). Chunk hashes change with every Render deploy, so find the code by the names and strings quoted here.
  - **The check:** in the `VariablesActionButtons` chunk, `ExternalUrlWarning` (`Zt`) and its check (`Xt`).
    - A value matching `rediss://[^@]+@[^:]+\.render\.com:\d+` or `postgresql://[^@]+@[^/]+\.render\.com/[^/\s]+`, whose region (read from the host) equals the service's, gets the warning.
    - Only while editing. It skips secret files, read-only rows, deleted rows and rows with no key.
  - **Strings:** "This variable references an External URL. Those should only be used from **services outside** of Render." The link opens `/postgresql-creating-connecting#external-connections` or `/key-value#enabling-external-connections`. Then **Use Internal URL**, "Review fix" (a diff of the old and new values), Cancel and **Update**.
  - **The rewrite:** `postgresql://{user:pass}@{host id}/{db}` keeping the query, and `redis://{host id}:{port}`.
  - **Where:** only the service's editor passes its region (`regionId`). Env group editors pass none and never warn. No flag gates it.
  - **Render's docs:** "Use the internal URL wherever possible". Internal URLs need the same workspace and region, and external ones "are slower because they traverse the public internet" ([postgresql-creating-connecting](https://render.com/docs/postgresql-creating-connecting), [key-value](https://render.com/docs/key-value)).
  - **Two places where bex builds the documented internal URL instead of copying Render's code:**
    - Render's Key Value rewrite drops the credentials, which breaks a store with internal auth on, whose internal URL Render's docs show with them.
    - Render's warning does not check environment isolation, so across an isolated environment's boundary it offers a URL the service cannot reach. This milestone matches Render there, and t006 files a guard as a separate decision for the user.
- **bex today (checked 2026-11-26):**
  - **Postgres:**
    - External URL: `postgresql://u:p@<id>.<BEX_DB_DOMAIN>:5432/<db>[?sslmode=verify-full]` (`lego/backend/internal/postgres/service.go:1216-1237`), with `-pool`, `-ro-<name>` and `-rp-<name>` host variants (`lego/operator/internal/controller/database_controller.go:2201-2223`).
    - Internal host: `<id>-rw.<namespace>.svc` (`database_controller.go:1315`), where the namespace is the workspace's.
  - **Key Value:** external `rediss://u:p@<id>.kv.bex.co:6379`, internal `redis://<id>:6379`, with credentials when internal auth is on (`lego/backend/internal/keyvalue/service.go:1539-1589`).
  - **Region:** no host carries a region. bex runs in one place (`BEX_REGION`), and a resource's region is a label set when it is created (`lego/backend/internal/resourcemeta/metadata.go:127-175`). So the check must look the datastore up rather than read its region from the text.
  - **What the external URL costs:** it goes through the public SNI edge, is metered as datastore egress ([ADR021](../../../docs/ADR021-keyvalue-management.md)), and is refused by a datastore's IP allowlist.
  - **What the dashboard can read:** the datastore's non-secret GraphQL read carries `region`, `externalHost`, `environmentId` and Key Value's `internalAuth` (`lego/backend/internal/postgres/graphql.go:68-87`, `lego/backend/internal/keyvalue/graphql.go:44-63`), but no Postgres internal host.
    - Reading a connection URL is an audited secret view, Admin-only in a protected environment (w4/m321, w4/m594). So the fix builds the new value from the pasted credentials and reads no secret.
  - **Today:** `dashboard/src/features/services/components/service-environment-editor.tsx:157` passes no region, and nothing shows the warning.
- **Not a duplicate of:** [w1/154](../blocked/154.md)'s picker, which remains the user's decision. No other item covers this.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)).
- **Expected outcome:** a service reaching its own datastore through the public edge gets Render's one-click fix, and stops paying egress and latency for it.
- **Why now:** the external URL is metered, slower, and refused once the datastore has an IP allowlist. The fix is Render's own and waits on no decision.
- **Render parity task:** included (t006). The dashboard's editor gains the warning, and GraphQL gains a bex-only field for a Postgres's internal hosts. REST and MCP do not change.
