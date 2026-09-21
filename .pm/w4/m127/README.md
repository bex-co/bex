# w4 · m127 — A protected environment refuses to pause a database and allows `DROP TABLE` inside it

**Worker:** worker4 **Goal:** a protected datastore is protected against the operations that actually destroy it — the data-plane console and the control-plane verbs that change durability, identity or topology — not only against the two that take it offline. **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | -------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Decide whether protection reaches the data plane, and record the answer           | 40m | —          |
| t002 | Guard the datastore control-plane verbs the hunt found open                       | 45m | t001       |
| t003 | Implement the data-plane decision for `executeDatabaseQuery(allowWrites:)`        | 40m | t001       |
| t004 | Render parity check (confirm argument + error shape across REST/GraphQL/MCP + UI) | 30m | t002, t003 |
| t005 | Simplify (`/simplify` over the changed code)                                      | 30m | t004       |
| t006 | Test coverage                                                                     | 45m | t004       |
| t007 | Closeout                                                                          | 15m | t006       |

## Definition of done

Each bullet is a probe run live in the hunt below, re-run against the fix with `qa-` datastores in a `protected` environment:

- **The data-plane decision is implemented and visible.** Whatever t001 decides, `executeDatabaseQuery(allowWrites: true)` on a protected database behaves according to a written rule: either it is refused without the confirmation phrase and accepts one, or ADR032 states why the guard deliberately stops at the control plane and the dashboard says so where a user can see it. Today it runs `CREATE TABLE`, `INSERT` and `DROP TABLE` with no confirmation and has no `confirm` argument to offer.
- **Durability cannot be turned off silently.** `setKeyValuePersistenceMode(persistenceMode: "off")` on a protected Key Value is refused without the phrase. Today it succeeds, on the same instance whose *suspend* is refused.
- **Eviction cannot be turned on silently.** `setKeyValueMaxmemoryPolicy(maxmemoryPolicy: "allkeys_lru")` on a protected Key Value is refused without the phrase.
- **Topology and identity are guarded.** `failoverDatabase` and `renameDatabase` on a protected database are refused without the phrase.
- **The existing guard still holds.** `suspendDatabase`, `suspendKeyValue`, `deleteDatabase` and `deleteKeyValue` still refuse without their phrases and still succeed with them — all four verified live in this hunt and not to be regressed.
- **Unprotected datastores are untouched**, and no verb gains a required argument.
- **Tests fail without the fix.** A test asserting `DROP TABLE` through the console on a protected database is refused fails against the pre-fix code.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` 2026-09-21 pass 142 (w4-targeted, `muse.env` credentials). A free-plan Postgres and a free-plan Key Value were created directly into a fresh `protectedStatus: protected` environment, and every reachable mutation was run against them. Full matrix in [t001](t001.md). The two halves:

  ```
  suspendDatabase            → REFUSED  sudo suspend database qa-20260921-db
  suspendKeyValue            → REFUSED  sudo suspend key value qa-20260921-kv
  deleteDatabase             → REFUSED  sudo delete database qa-20260921-db2
  deleteKeyValue             → REFUSED  sudo delete key value qa-20260921-kv

  executeDatabaseQuery allowWrites CREATE TABLE qa_probe(x int)   → ACCEPTED
  executeDatabaseQuery allowWrites INSERT INTO qa_probe VALUES(1) → ACCEPTED
  executeDatabaseQuery allowWrites DROP TABLE qa_probe            → ACCEPTED
  setKeyValuePersistenceMode  "off"        → ACCEPTED
  setKeyValueMaxmemoryPolicy  "allkeys_lru"→ ACCEPTED
  failoverDatabase                         → ACCEPTED (true)
  renameDatabase                           → ACCEPTED (renamed)
  ```

  The four refusals are the control: the guard was armed throughout, and the cleanup went through it with the phrases. All fixtures (database, Key Value, environment, project) deleted; the workspace is back to its 3 databases / 2 Key Values / 3 projects / 5 services.
- **Goal linkage:** tenant safety on the datastore surface and the honesty of the protection control — [ADR032](../../../docs/ADR032-environments.md) § `protectedStatus`, [ADR018](../../../docs/ADR018-render-parity.md) Environments row. `w6/m19` built the guard for services; `w6/m37` extended it to Postgres and Key Value, closing a gap m19's own closeout had flagged. This is the next layer of the same question, and one part of it is genuinely new: m37 asked *which resources* the guard covers, and this asks whether it covers the **data plane** at all.
- **Expected outcome:** marking an environment protected stops meaning "you may not pause this database, but you may empty it". A user who turns protection on gets a coherent answer from the operations that can actually lose their data.
- **Why now:** the asymmetry is at its widest exactly where the consequences are worst. Suspending a database is reversible in one click; `DROP TABLE` and `persistenceMode: "off"` are not, and both are accepted today on an instance whose suspend is refused. The mechanism to fix it already exists and is already wired into these very packages — `postgres/protection.go:32` and `keyvalue/protection.go:32` each hold a `requireUnprotected` with exactly two call sites.
- **Relationship to [`w4/m126`](../m126/README.md):** that milestone covers the same shape for *service* verbs (`setImage` on a protected service). t002 here is the datastore analogue and should reuse whatever rule m126/t001 settles rather than inventing a second one. t003 is not covered by m126 — no service verb executes tenant-supplied code against tenant data the way the SQL console does.
- **Render parity task included** because it adds a `confirm` argument and a coded refusal to datastore verbs across REST, GraphQL and MCP, and the dashboard must retry with the phrase.
- **DO_NOT_DO constraints honored:** no anti-goal touched; the milestone only adds refusals and an opt-in confirmation, and removes no capability.
