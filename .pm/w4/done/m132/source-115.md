# Postgres parameter overrides accept any name and any value, then silently never apply the invalid ones

Why: `setDatabaseParameterOverrides` performs no validation — a parameter Postgres has never heard of, and a nonsense value for a real one, are both stored and read back as declared configuration — so the declared surface reports settings the running server does not have and nothing anywhere says the request was not honored.

Found by live `/qa-find-bugs` 2026-09-21 pass 122 (w4-targeted, `muse.env` credentials) on throwaway Postgres `qa-20260921-pgp` (`dpg-daoge2h2dbts73fi2mf0`), since deleted.

## Repro

```
mutation { setDatabaseParameterOverrides(id:"dpg-…", parameters:[{name:"work_mem", value:"not-a-size"}]) { id } }
→ 200, no error

query { databaseParameterSpec(id:"dpg-…") { name value } }          # declared
→ [ { name: "work_mem", value: "not-a-size" } ]                      # stored and reported

query { databaseParameterOverrides(id:"dpg-…") { name setting } }    # observed, from pg_settings
→ 48 parameters, and work_mem is NOT among them                      # never applied
```

Same for a name Postgres does not have:

```
parameters:[{name:"qa_not_a_real_parameter", value:"1"}]
→ 200; declared reports it; observed never contains it
```

The control proves the path works when the input is valid: `{name:"work_mem", value:"8MB"}` reaches the server and reads back on the observed surface as `work_mem = 8192` with `source: "configuration file"` — correctly normalized into Postgres's base unit.

## What this is, and what it is not

**It is:** accepted-and-never-applied configuration, the same class as `w4/m120` (a write the API takes and the runtime ignores). The declared surface is the one a user reads back to confirm their change, and it confirms something untrue.

**It is not an outage, and I want that on the record because a mid-probe reading suggested otherwise.** After setting `max_connections: 5`, `executeDatabaseQuery` returned `internal error`, the observed parameter list went empty, and `database.status` still read `available` — which looks exactly like "the API accepted a value that took the database down and then lied about it". It is not: a later probe found the database already healthy **before** anything was reverted (`sqlOk: true`, 48 observed parameters). The failure window was the ordinary Postgres restart that applying a parameter change requires. No override tested here left the database unhealthy.

Worth a separate look, but not claimed here: during that restart `status` stayed `available` while in-cluster reads failed. A brief restart is expected for a config change, so this may be correct-by-design; it was not investigated.

## Where to fix

`setDatabaseParameterOverrides` (`lego/backend/internal/postgres/`) writes the declared set through with no check. The options, in increasing cost:

- **Validate the name** against the parameter catalogue the platform already knows — `databaseParameterOverrides` reads live `pg_settings`, so the set of legal names is available at the server, and `databaseParameterSpec`/`list_postgres_parameters` already distinguishes declared from observed (ADR018, `w6/m133`).
- **Validate the value** by type/unit from the same catalogue, so `work_mem: "not-a-size"` is a named 400 rather than stored fiction.
- **Or surface the divergence** — if pass-through is deliberate (Postgres is the real validator and refusing early would block legitimate future parameters), then the declared read should mark an entry the observed set does not corroborate, so the user can see their override never landed.

The third is the cheapest honest fix and does not require bex to own a parameter catalogue. Whichever is chosen, say which in the change, because "validate everything" and "pass through and report" are different products.

Estimate: ~40m for the divergence signal; ~1.5h if name+value validation is wanted, and then it deserves a milestone rather than this note.

## Unverified

- Whether a declared-but-invalid override blocks a _later valid_ one from applying was not tested.
- Whether the same pass-through exists on the Key Value config surface — `setKeyValueMaxmemoryPolicy` and `setKeyValuePersistenceMode` both validate strictly (pass 120 saw them refuse unknown values with the full valid set enumerated), so the two datastore surfaces appear to disagree about whether to validate. That comparison is worth making part of the fix.

## Also checked this pass, and found correct

- **Valid overrides work end to end**, declared and observed, with correct unit normalization (`8MB` → `8192`).
- **The declared/observed split is coherent** and matches what ADR018 records: `databaseParameterSpec` is what you asked for, `databaseParameterOverrides` is what `pg_settings` reports, with `source` and `unit`.
- **REST agrees with GraphQL** on the declared set: `GET /v1/postgres/{id}/parameters` → `[{"name":"work_mem","value":"8MB"}]`.
- **Clearing overrides works** — `parameters: []` restores the database to 48 observed parameters and no declared set.
