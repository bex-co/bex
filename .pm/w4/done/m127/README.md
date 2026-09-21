# w4 · m127 — A protected environment refuses to pause a database and allows `DROP TABLE` inside it

**Worker:** worker4 **Goal:** a protected datastore is protected against the operations that actually destroy it — the data-plane console and the control-plane verbs that change durability, identity or topology — not only against the two that take it offline. **Status:** done 2026-09-21 (live re-probe of the deployed fix deferred to the next QA pass — no production access this session)

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | -------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Decide whether protection reaches the data plane, and record the answer           | 40m | —          | — **DONE**
| t002 | Guard the datastore control-plane verbs the hunt found open                       | 45m | t001       | — **DONE**
| t003 | Implement the data-plane decision for `executeDatabaseQuery(allowWrites:)`        | 40m | t001       | — **DONE**
| t004 | Render parity check (confirm argument + error shape across REST/GraphQL/MCP + UI) | 30m | t002, t003 | — **DONE**
| t005 | Simplify (`/simplify` over the changed code)                                      | 30m | t004       | — **DONE**
| t006 | Test coverage                                                                     | 45m | t004       | — **DONE**
| t007 | Closeout                                                                          | 15m | t006       | — **DONE**

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

## Outcome (2026-09-21)

**t001 — the decision, and the check that settled it.** The task offered two defensible answers and asked for one, written down. It also named the test that decides between them: *"check whether protection blocks anything at the connection-string level. If it does not, the confirmation genuinely is advisory."*

It does not, and cannot. `protectedStatus` is a control-plane mechanism; a caller holding `can_create` + `can_view_sensitive` can read the connection string and run the same `DROP TABLE` over the wire, where no bex-api guard exists. So the second option's argument — *"an API-level confirmation is theatre against anyone determined"* — is **correct on its own terms**, and the decision still goes the other way, because it answers the wrong question. The dominant failure here is a mis-aimed console tab, not a determined attacker. A confirmation is worth having against accident.

So: **protection covers the data plane**, and ADR032 says in the same breath that the data-plane confirmation is an **accident guard, not a security boundary** — which is the honest version of option two's insight, kept rather than discarded. Implying a containment the mechanism cannot deliver would have been the worse outcome of the two.

**t002/t003 — the guarded set, and the rule behind it.** Delete and suspend were the whole set; it is now delete, suspend, writable SQL, failover, rename, major-version upgrade, durability (`persistenceMode`) and eviction (`maxmemoryPolicy`). Resume stays exempt as it always has — protection blocks taking availability away, not restoring it — and a **read-only query is never gated**, because protection has no reason to stop the one console use that cannot lose anything.

Two verbs beyond the DoD's list are included, with reasons rather than by sweep: a **major-version upgrade** takes the database offline for `pg_upgrade` and cannot be undone, which is strictly more destructive than the rename the DoD does name; and a **Key Value rename** is the identity twin of the Postgres rename, so leaving it out would have made the two datastores disagree. ADR032 states the rule that decides membership — *if the verb can lose data, change what the resource IS, or take it offline, it needs the phrase* — so the next field is placed without another live matrix. A plan change is billing and an allow-list edit is a firewall; neither qualifies, and both are asserted untouched.

Guards live in the **service verbs**, not the adapters, so REST, GraphQL and MCP are covered by construction rather than three times. `protectedDatabasePatchVerb` / `protectedKeyValuePatchVerb` pick **one** verb per call, most-destructive first, so a patch that both renames and upgrades asks for the upgrade phrase — a caller cannot satisfy the cheaper one and get both.

**t004 — parity and plumbing.** `confirm` is optional on every affected surface and **no verb gained a required argument** (a DoD bullet). REST reads `?confirm=`, and the query route also accepts it in the body since an SQL statement does not belong in a URL. GraphQL got it through `gqlutil.PatchMutation`/`ArgMutation` rather than per-mutation, which covers every single-field setter at once and means a newly-guarded setter cannot ship without a way to confirm it. MCP reuses the existing confirmation-carrying args struct rather than advertising the field on unrelated tools. The error shape is unchanged — the same `core.RequireEnvironmentConfirmation` phrase every guarded verb has always returned.

**t006 — coverage**, mutation-spot-checked by reverting each guard: `TestProtectedDatabaseDataPlaneAndTopology` (the DoD's explicit `DROP TABLE` case — refused, nothing reaching the database, then accepted with the phrase and observed executing; plus read-only never gated, failover not stamping the spec on refusal, rename/upgrade, and an unprotected database gaining no ceremony) and `TestProtectedKeyValueDurabilityEvictionAndIdentity` (durability, eviction, rename — each asserting the spec did not move on the way to the refusal — plus the unguarded-field and unprotected-store controls).

**Green:** `lego/backend` `go test ./...` all packages + `golangci-lint` 0 issues. The four pre-existing refusals the hunt used as its control (`suspendDatabase`, `suspendKeyValue`, `deleteDatabase`, `deleteKeyValue`) still pass their original tests unchanged.

**Not done — the live re-probe, and one surface.** Every DoD bullet is a probe against `qa-` datastores in a protected environment and none has been re-run; no production access this session. The **dashboard** was not touched: the SQL console's own confirmation is a client-side dialog that predates this and is unaffected by the API gaining a `confirm` argument, but a protected database will now refuse a writable query the console does not yet send a phrase for. **That is a real gap for the next pass** — the console needs to surface the server's refusal and offer the phrase, the way the delete and suspend dialogs already do. It is filed here rather than half-built blind.
