# w9 · m87 — Finish the role-aware disable-with-reason sweep

**Worker:** worker9 **Goal:** no contributor-facing dashboard control that is a `can_create` write is still "editable → 403 on save" — env-var writes, the Build Filters editor, and the datastore lifecycle danger zone are disabled-with-reason for a role that lacks the relation, completing the `w9/m84` sweep. **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on                            |
| ---- | --------------------------------------------------------------------------------- | --- | ------------------------------------- |
| t001 | Gate env-var writes (add/edit/delete) in the shared `EnvVarsEditor`               | 45m | —                                     — **DONE** |
| t002 | Gate the Build Filters editor (`BuildFilterEditor` → `setBuildFilter`)            | 30m | —                                     — **DONE** |
| t003 | Gate the datastore lifecycle danger-zone controls (delete PG/KV, plan change)     | 40m | —                                     — **DONE** |
| t004 | Render parity (UI role-gating consistency)                                        | 20m | [w9/m87/t001, w9/m87/t002, w9/m87/t003] — **DONE** |
| t005 | Simplify                                                                          | 20m | [w9/m87/t004]                         — **DONE** |
| t006 | Test coverage (per-role component tests for each gated surface)                   | 40m | [w9/m87/t004]                         — **DONE** |
| t007 | Closeout                                                                          | 10m | [w9/m87/t006]                         — **DONE** |

## Definition of done

For a **contributor** (no `can_create`):

- every env-var **write** control — Add-variable form, the `env-var-row.tsx` Edit pencil and Delete, across **both** the service Environment tab and the workspace **env-groups** editor (live shared `EnvironmentEditor`; leftover `EnvVarsEditor` gated the same way) — renders **disabled with an explanatory reason** (`PermissionTooltip`) instead of an editable control that 403s on save;
- delete Postgres / delete Key Value are disabled-with-reason.

`SetBuildFilter` and datastore `SetPlan` are **`can_operate`** on the server (inbox guessed `can_create`). The UI gates to that relation: **viewers** see those editors disabled-with-reason; **contributors keep them**. Restart/suspend/resume stay `can_operate` — `w2/m74` already gated those on the datastore danger zone (this milestone left them to that work).

Landing overlapped `w2/m74` (on origin/main): live `EnvironmentEditor` writes/reveal and datastore delete/plan/lifecycle were already shipped there. This milestone's remaining unique surface is leftover `EnvVarsEditor` write gating plus Build Filters (`can_operate`).

For an **admin** all the above render enabled and function as today. Per-role component tests assert both states for each surface. `yarn typecheck && yarn lint && yarn test` green.

## Source + Goal linkage

- **Source:** `.pm/w9/048.md` (filed at `w9/m84` closeout, 2026-08-17; promoted via `/pm-brainstorm for w9` 2026-08-18). Note now moved to `w9/done/048.md`.
- **Goal linkage:** V0 pillar 5 (multi-tenant + enforced authz) made legible in the UI — the dashboard tells the truth about what a role can do **before** the server refuses, extending `w9/m84`'s `useCapabilities` + disable-with-reason plumbing to the `can_create` controls it left for scope.
- **Expected outcome:** no contributor-facing dashboard control that is a `can_create` write is still "editable → save fails 403"; the role-gating story `w9/m84` began is complete and uniform across env-vars, build filters, and datastore lifecycle.
- **Why now:** it is a coherence residual of a just-shipped milestone (`w9/m84`) while the exact pattern (`useCapabilities` + `disabled`/`disabledReason` / `PermissionTooltip`) is warm and the four already-gated controls are the working template — cheap to finish now, drifts and gets re-discovered later.
- **Render parity — included (light/UI):** no REST/GraphQL/MCP wire change (the server relations are already the authority), but the change touches the `dashboard/` surface, so the parity task confirms the disabled-with-reason treatment matches Render's own contributor handling (Render disables/hides these similarly for non-admins) and flags any drift as follow-up rather than diverging silently. Live Environment-tab reveal was already gated by `w2/m74`.
