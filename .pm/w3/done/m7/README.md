# w3 · m7 — Service events feed (`GET /v1/services/{id}/events`)

**Worker:** worker3 **Goal:** The activity feed the parity matrix parked behind two prerequisites that are both now done — deploy objects (w2/m5) and the audit log (w4/m10): one paged, newest-first events surface per service composing deploy transitions, lifecycle/config writes, and scale/sleep transitions, under Render's `GET /services/{id}/events` shape. **Status:** done

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Event model + store view: compose deploys + audit_events + scale/sleep transitions; derived-vs-recorded per type  | 35m | —          | — **DONE** |
| t002 | REST `GET /v1/services/{id}/events` (Render envelope/cursor, shapes vs OpenAPI) + GraphQL `serviceEvents`         | 30m | t001       | — **DONE** |
| t003 | MCP: mirror the official server's events tool if one exists, else document the omission                           | 15m | t002       | — **DONE** |
| t004 | Acceptance: suspend + deploy + scale a service → feed shows each, newest-first, paged; no secrets                 | 25m | t002       | — **DONE** |
| t005 | Render parity — surface consistency + Render event-type comparison; matrix events row ✖ → status                  | 20m | t003, t004 | — **DONE** |
| t006 | Simplify — `/simplify` over the code this milestone changed                                                       | 20m | t005       | — **DONE** |
| t007 | Test coverage — composition correctness, cursor stability, redaction, store-less 503                              | 30m | t005       | — **DONE** |
| t008 | Closeout — DoD met → move milestone to `done/`                                                                    | 10m | t007       | — **DONE** |

## Definition of done

A service's events endpoint returns a truthful, cursor-paged, newest-first feed whose entries correspond 1:1 to real deploys, lifecycle/config verbs, and scale/sleep transitions — mapped onto Render's event vocabulary where one exists, bex-named where it doesn't (documented); env-var and secret values never appear in any event; store-less mode (`BEX_CP_DB_URI` unset) returns 503 (omitted, not faked); the matrix's "Service events / activity feed" row moves from ✖. **Met** — proven live by `scripts/events-verify.sh` (exit 0): a scripted create → suspend → resume → scale → env-var write → deploy sequence appears exactly once each, newest-first across both sources, pages with `limit=2` with no duplicate or gap, and the planted env-var value appears in no response on any surface (while still being readable via `GET /env-vars`, so the omission is redaction, not a broken write).

## What shipped

**Composed, not recorded.** The feed is a read-time VIEW (`lego/backend/internal/events`, one `UNION ALL` in `internal/store/events.go`) over two tables bex already writes — `deploys` (→ `deploy_started`/`deploy_ended`) and `audit_events` (→ every other type). No event table, no second write path. The whole cost of making it possible was **one column**: `audit_events.target`, the resource a verb acted ON, written at the same single interception point every write verb already passes (`core.Base.AuthorizeTarget`, the new sibling of `Authorize`/`AuthorizeOn`). `resource` says what a verb was authorized *against* (the workspace); `target` says what it *changed*.

**Surfaces.** REST `GET /v1/services/{id}/events` (verified field-by-field against Render's OpenAPI `list-events`: bare `[{event, cursor}]` array, `event = {id, timestamp, serviceId, type, details}`, params `type`/`startTime` **defaulting to now-1h, Render's own default**/`endTime`/`cursor`/`limit`), GraphQL `serviceEvents(serviceId, …)`, and MCP `list_service_events`. All three go through one `Service.List`; `TestEventSurfaceParity` holds them to identical output.

**Decisions worth remembering (t001/t003/t005):**

- **Derived vs recorded:** everything is derived. The scale/sleep question resolved to an honest **omission**: auto-sleep/wake and autoscaler-driven replica changes are driven by the operator, which is DB-free by architecture (ADR003) and records no Kubernetes Events — so they have no durable source, and recording them would mean giving mechanism a control-plane write path (a layering inversion, not a light write). Manual scale IS covered. Documented, not faked.
- **Redaction is structural, and it has a price we paid deliberately:** an audit row carries a verb NAME, a caller, and a target NAME — never a verb's arguments. So `plan_changed` / `instance_count_changed` / `autoscaling_config_changed` **omit** the `from`/`to` fields Render marks *required*. Carrying them would mean a free-form details column on `audit_events` — exactly the hole w4/m10 closed to make "no value can reach an event" a property of the schema rather than a filter someone must remember.
- **MCP is a bex extension, not a mirror:** Render's official MCP server (checked `2a00be1`, 2026-07-12) registers 24 tools and has **no events tool at all** — so there was no name to mirror and no parity gap. bex adds one anyway, in Render's tool grammar, because an agent can otherwise only poll `list_deploys`, which sees rollouts and nothing else.
- **Ids are `evt-…` but derived**, not minted (`id.Derive`, new in `internal/id`): an event is a projection, so a fresh xid per read would break every client's paging and dedupe. The cursor carries the keyset rather than a row id, so it still pages correctly after the audit retention sweep deletes the row it named.

**Two bugs found along the way, one fixed here and one filed:**

- `scripts/auth-bootstrap-client.sh` "failed" after succeeding — under `set -e` a trap whose last command returns non-zero makes the whole script exit non-zero. Fixed (it also broke `secrets-verify.sh`).
- **bex-api never exits on SIGTERM** — `ctrl.SetupSignalHandler()` catches it and cancels the context (stopping the projector, the retention sweeps and the internal API) while `log.Fatal(ListenAndServe())` keeps serving. In Kubernetes that is a full 30s grace period of a terminating pod taking traffic with its control plane already shut down. Out of scope here; filed as **`.pm/w1/019.md`**.

**One landmine defused:** migration numbering. Two earlier collision fixes renumbered migrations *downward*, so databases migrated before them record a `schema_migrations` version HIGHER than the file count (the local mock cluster's `bex-db` sat at 11 with ten migrations applied). A file numbered `0011` is silently skipped on such a database — and since an audit `Record` error is swallowed by design, the symptom would not have been an error but a **permanently empty events feed**. The migration is numbered **0012**, which converges both populations; the reasoning is in the file so 11 stays burned.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12; docs/ADR018-render-parity.md row "Service events / activity feed" (✖, previously cross-referenced to "w2/m5 (deploy objects) + w4/m10 (audit log)" — both done, making this a composition milestone); Render `GET /services/{id}/events` (OpenAPI).
- **Goal linkage:** GOAL.md #2 (observability — "what happened to my service") + pillar 1 (Render service-detail parity); pillar 3 (agents reason over activity, not just current state).
- **Expected outcome:** "what happened to this service overnight?" is one API call; the last unowned observability row in the matrix's Services section gets an owner. **Achieved** (pass `startTime` — the default window is Render's one hour).
- **Render parity closing task: included** — REST/GraphQL/MCP surface; matrix row ✖ → ◐ REST / ✅ GraphQL / ✅ MCP / ✖ UI, with the divergence list as evidence. The dashboard **Events tab** is deliberately out — filed as `w5/007` (shared with w2/m10's deploy-list buttons), per the UI-half pattern.
