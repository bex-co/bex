# w6 · m112 — The audit log cannot say what was acted on: GraphQL drops the target, and one user action writes four rows

**Worker:** worker6 **Goal:** every audit row names the object it acted on, on the surface the dashboard actually reads, and one user action produces one record. **Status:** todo

## Background (found live, 2026-08-27, 23rd `/qa-find-bugs` run)

Ran journey 1 end to end on production as a real user would: created project `qa-20260827-proj` (`prj-da7qdbnkrsvc73c3m2cg`), created environment `qa-20260827-env` (`env-da7qdivkrsvc73c3m2eg`) inside it, and assigned one service (`srv-da7o6ovvqdcc73bpn9hg`) to that environment through **Manage resources → check one box → Save**. The journey's own promise held everywhere — the Overview moved the service out of "Ungrouped Resources", the project card appeared, the service's breadcrumb read `qa-20260827-proj > qa-20260827-env > qa-20260826-webhook-renamed`, and REST agreed (`projectId`/`environmentId` both set). All three resources were deleted afterwards.

Then read back what those three mutations left in the audit log. That is where it falls apart.

### Defect A — GraphQL's audit row has no target field at all

REST carries it. In-page authenticated probe (`fetch(..., {credentials:'include'})` from inside `dashboard.bex.co`, per this hunt's Phase-3 trap about bare-UA clients getting Cloudflare `1010`):

```
GET https://api.bex.co/v1/owners/tea-d98210cbbpdc73dcrkvg/audit-logs?limit=100&startTime=<now-1h>

{ "auditLog": {
    "id": "aud-da7qe27krsvc73c3m2jg",
    "timestamp": "2026-08-27T02:55:04Z",
    "event": "environments.SetServices",
    "status": "success",
    "actor": { "type": "user", "id": "c73bb20d-…" },
    "metadata": {
      "relation": "can_create",
      "service": "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc"   ← the target, present
    },
    "resource": "workspace:tea-d98210cbbpdc73dcrkvg" },
  "cursor": "aud-da7qe27krsvc73c3m2jg" }
```

GraphQL does not:

```
query { __type(name: "AuditLog") { fields { name } } }
=> action · actor · actorMethod · id · metadata · oauthAudience · oauthClientId
   · oauthScopes · relation · resource · status · targetName · timestamp
   — 13 fields, no `target`

query { __type(name: "AuditLogMetadata") { fields { name } } }
=> ["to"]        — the maintenance-mode boolean, and nothing else

query { auditLogs(ownerId: "tea-d98210cbbpdc73dcrkvg", limit: 40) {
          timestamp action targetName resource metadata { to } } }
=> every mutation row this session:
   { "action": "environments.SetServices",   "targetName": null, "metadata": null,
     "resource": "workspace:tea-d98210cbbpdc73dcrkvg", "timestamp": "2026-08-27T02:55:04Z" }
   { "action": "environments.CreateWithACL", "targetName": null, "metadata": null, … }
   { "action": "CreateProjectEvent",         "targetName": null, "metadata": null, … }
```

`resource` is the **workspace**, not the object. So over GraphQL there is no way to learn which environment, which project, or which service any row refers to — and `dashboard/src/features/audit/api/audit.graphql` is GraphQL-only, so the dashboard's audit table structurally cannot name the object either.

**The resolver's own comment describes a fallback that cannot exist.** `lego/backend/internal/audit/graphql.go:70-77`:

```go
// targetName is the target's stored display name (migration 0038);
// null for pre-0038 rows (stored "") so the dashboard falls back to
// the raw id rather than rendering an empty cell.
"targetName": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e Event) any {
    if e.TargetName == "" { return nil }
    return e.TargetName
})},
```

The raw id is `Event.Target` (`lego/backend/internal/audit/service.go:68`), which **is** stored (`lego/backend/internal/store/audit.go:116`, column `target`) and **is** read back into the domain type (`service.go:94`) — it is simply never exposed as a GraphQL field. There is nothing for the dashboard to fall back **to**. This is the probe contradicting the code, and the code is what is wrong.

**Honest scoping — the dialect difference is deliberate; this gap is not.** `w4/m26` (done, 2026-07-15) states: _"REST aligned exactly — … required string-map metadata keyed by target kind via new `core.SplitTarget` … **GraphQL deliberately keeps bex's flat dialect for the dashboard**"_. So GraphQL being flat rather than Render-shaped is a settled decision and must stay. What that decision never said is that the flat dialect should carry **no target at all** — and `graphql.go`'s comment proves the author believed the id was reachable. The fix is a flat `target` field (and/or a populated `targetName`), not a move to Render's envelope.

### Defect B — one action, four identical audit rows

The single **Save** that assigned one service wrote **four** `environments.SetServices` rows, same second, same actor, same `metadata.service`, four distinct ids (`…m2jg`, `…m2j0`, `…m2ig`, `…m2i0`). Creating the environment wrote **two** `environments.CreateWithACL` rows (`aud-da7qe27krsvc73c3m2…` pair at `02:54:03Z`). Creating the project wrote one.

Four rows is not per-resource-kind fan-out dressed differently: all four carry the identical action name and the identical single `metadata.service` value, so each is a full record of the same change, not a per-tab shard naming its own kind. Whatever the cause, the audit log multiplies one user action into four, which is exactly the property an audit log must not have.

### Defect C — `projects.Create` records no target on any surface

`CreateProjectEvent`'s REST row is `"metadata": { "relation": "can_create" }` — no project key, no name. So unlike `environments.SetServices`, even REST cannot say which project was created. Distinct from Defect A (which is a GraphQL exposure gap over a populated column); this one is a producer that never sets `Target` at all. `lego/backend/internal/audit/service.go:141` maps `"projects.Create" → "CreateProjectEvent"` — the Render event-name mapping is correct and deliberate, so the action name is **not** the defect; the empty target is.

## Target behavior (named)

- **A:** a `target` field on the GraphQL `AuditLog` type carrying the stored `Event.Target` verbatim (`"service:tea-…-qa-…"` / `"project:prj-…"`), keeping the flat bex dialect `w4/m26` settled on. `targetName` stays the optional display name layered on top. The dashboard then renders the object for every row, and `graphql.go`'s comment becomes true instead of aspirational.
- **B:** one user-visible mutation ⇒ one audit row.
- **C:** `projects.Create` sets `Target` to the created project, so REST's `metadata` names it the way `environments.SetServices` already names its service.

## Blast radius

- `Event.Target` is populated by a shared path (`core.verbAuditEvent` + per-verb setters). `grep -rn "TargetName:\|TargetName =" lego/backend/internal --include='*.go' | grep -v _test` returns **10** setters — t001 must extend that count to the `target` side and record it, since which verbs set `Target` at all is the thing Defect C turns on and this run measured it for only three verbs.
- Audit reaches callers through `lego/backend/internal/audit/{graphql,rest}.go` — MCP exposure not checked this run (see Unverified). Adding a GraphQL field is additive and cannot break REST.
- Verbs whose rows look fine today (`environments.SetServices` over REST) are fine **because** their producer sets `Target`; they are the regression surface for C's fix, not the control.

## Adjacent classes

Audit rows already carry a denial class (`status: "denied"`, `audit/graphql.go:62-67`). Exposing `target` must not turn the audit log into an existence oracle: the log is admin-scoped per workspace (`RelCanManage`, per `audit.graphql`'s own header), so a reader already holds workspace-manage rights over every object it can name — state that explicitly in the fix rather than assuming it, and confirm a denied row's target is safe to show to the same audience.

## Look-alike symptoms traced separately (not folded in)

- **Read-only `List` verbs dominate the log** — `agentsessions.List`, `apikeys.ListAPIKeys`, `sshkeys.List` and `audit.List` fill 18 of the 25 newest rows, and `audit.List` audits itself so reading the log grows it. Initially mis-attributed to dashboard polling; the REST capture disproves that — `agentsessions.List` rows carry `actor.type: "rest_api"` with `oauthClientId: "bex-mobile"`, `oauthAudience: "https://api.bex.co/mcp"`, i.e. a different client entirely. Whether read-auditing at this volume is intended is its own question with its own cause. Recorded, not filed.
- **Write→read propagation lag.** `setEnvironmentServices`/`setProjectServices` returned success but REST still reported the old `projectId`/`environmentId` 1.5s later; the same read was correct at 11.5s. Not filed — this run did not establish an upper bound or a promised one, and a mutation-then-immediate-read is not a contract anyone stated.

## Unverified (carried forward)

- Whether MCP exposes the audit log, and if so whether it carries the target — not probed this run.
- The cause of Defect B. Four rows was observed, not traced; t002 must find the call path before proposing a fix.
- Whether verbs beyond the three exercised here (`environments.SetServices`, `environments.CreateWithACL`, `projects.Create`) set `Target` — three verbs is not the vocabulary.
- Whether the dashboard's audit table would render a `target` field usefully once it exists, or needs its own column work.

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Expose the stored target on GraphQL's flat `AuditLog`, and count which verbs set it     | 45m | —          |
| t002 | Trace and fix the 4×/2× duplicate audit rows for one user action                        | 45m | —          |
| t003 | Give `projects.Create` a target so REST's metadata names the project                    | 30m | t001       |
| t004 | Render parity — audit log across REST/GraphQL/MCP and the dashboard table               | 30m | t001, t002, t003 |
| t005 | Simplify — `/simplify` over the code this milestone changed                             | 20m | t004       |
| t006 | Test coverage                                                                           | 40m | t004       |
| t007 | Closeout                                                                                | 15m | t006       |

## Definition of done

Each bullet is a probe or a click the next person can repeat and watch pass or fail.

1. Create a project, an environment, and assign one service to it (the exact journey above). Then `query { auditLogs(ownerId: "<ws>", limit: 20) { action target targetName resource } }` returns a non-null `target` naming the acted-on object for **each** of the three rows. Today the `AuditLog` type has no `target` field at all.
2. The dashboard's audit table displays that object for those rows instead of leaving the reader with only `workspace:<id>`.
3. The same three mutations produce **three** audit rows, not seven. Today: 1 project + 2 environment-create + 4 set-services.
4. `GET /v1/owners/<ws>/audit-logs` for the project-create row returns `metadata` containing a project key, alongside the `relation` it already carries. Today it is `{"relation": "can_create"}` only.
5. REST's existing shape is unchanged for `environments.SetServices` — its `metadata.service` still reads `tea-…-qa-…`, proving the fix was additive on GraphQL and did not migrate REST off the shape `w4/m26` settled.
6. The count of verbs that set `Target` is recorded in this README, and every verb this milestone touched has a test asserting its target.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 23rd run, 2026-08-27, journey 1 (project/environment create + assign). Probes and their complete responses are pasted inline — for an API contract the durable artifact is the request and the response, not a screenshot; `.playwright-mcp/` captures are gitignored and session-local and nothing here rests on them. All three `qa-` resources were deleted at the end of the run and the borrowed service returned to unassigned (verified: `projectId`/`environmentId` both absent, phase `Running`).
- **Goal linkage:** `docs/ADR006-bex-api.md` § Audit log and `w4/m26`'s Render-shape verification own this surface; ADR012 (auth) and ADR024 (members) depend on the audit log being able to answer "who changed what" — today it answers only "who did something, somewhere in this workspace".
- **Expected outcome:** the workspace audit log becomes usable for its one job — attributing a change to an object — on the surface the dashboard reads, with one row per action.
- **Why now:** the missing target is invisible in code review (the resolver comment asserts a fallback that reads as correct) and only shows up when someone actually reads the log after a change. The duplicate rows compound it: a reader who cannot tell what changed also cannot tell how many times it changed.
- **Render parity task included:** yes — the change adds a GraphQL field on a surface with REST and possibly MCP siblings and alters what the dashboard renders. `w4/m26` deliberately diverged GraphQL from Render's envelope; t004 must confirm this fix stays inside that decision rather than reopening it.
