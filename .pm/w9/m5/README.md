# w9 · m5 — Secret-file list pagination

**Worker:** worker9 **Goal:** `GET /services/{id}/secret-files` honors Render's `cursor`/`limit` contract (today the params are accepted and ignored), and the GraphQL/MCP/dashboard secret-file lists page to match — the w9/m10 stable-name cursor pattern applied to env vars' sibling route. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | REST: honor `cursor`/`limit` with the m10 stable-name cursor; conformance suite clean          | 30m | —          |
| t002 | GraphQL/MCP pagination args to match (the m10 shapes); dashboard list follows                  | 40m | t001       |
| t003 | Boundary tests (first/last/expired cursor, over-limit) + a verify-script leg if the CLI exercises the route | 20m | t002       |
| t004 | Render parity — param semantics vs Render's env-vars pagination; three-surface consistency     | 20m | t003       |
| t005 | Simplify — `/simplify` over the code this milestone changed                                    | 20m | t004       |
| t006 | Test coverage — meaningful pagination tests beyond t003's boundaries (stability under mutation) | 20m | t004       |
| t007 | Closeout — DoD met → move milestone to `done/`                                                 | 10m | t006       |

## Definition of done

The secret-files list pages identically to env vars on all three surfaces: `limit` respected, `cursor` resumes correctly (stable across interleaved writes per the m10 stable-name property), the ignored-params behavior is gone (nothing accepted is ignored), and the conformance suite validates the route without an allowlist entry.

## Source + Goal linkage

- **Source:** promotes `w9/005` (filed by round 7's field-diff: Render's OpenAPI gives the route the same `cursor`/`limit` contract as env vars; bex emits `{secretFile, cursor}` items but ignores the params; GraphQL/MCP/dashboard unpaged).
- **Goal linkage:** Render parity (pillar 1); finishes the pagination family `w9/m10` (env vars) and `w2/m31` (deploys) established.
- **Expected outcome:** one more accepted-but-ignored param class deleted; the route joins the conformance-guarded set honestly.
- **Why now:** the m10 cursor mechanism is fresh in-tree — applying it to the sibling route is the cheapest it will ever be; the note explicitly scopes away env-group content pagination, keeping this small.
- **Render parity closing task: included** (t004) — REST/GraphQL/MCP list shapes change.
