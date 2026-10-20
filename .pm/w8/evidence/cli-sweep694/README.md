# CLI sweep 694 — REST `/logs` filter permissiveness (live, 2026-10-19)

Read-only against the baseline `hello-go`. (The `task`/`taskRun` filter family is w8/132, awaiting the production pin — not re-probed.)

- `GET /logs?resource=<baseline>&limit=2` → 200 `{hasMore:true, nextStartTime, nextEndTime, logs:[…]}` (Render page envelope).
- `level=bogus` → 200 with `logs:[]` — the REST core is permissive (unknown filter values match nothing, the same posture ADR018 records for GraphQL/MCP event types); the pinned CLI refuses `bogus` client-side with its enum (sweep 661). Layer disagreement is documented design, not a silent drop of valid values.
- Missing `resource` → 400 `invalid query parameter "resource"`. `direction=forward` pages normally.

No bug filed.
